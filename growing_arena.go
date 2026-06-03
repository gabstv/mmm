package mmm

import "unsafe"

// GrowingArena is a linear bump allocator that adds Chunks on demand when the
// active chunk is exhausted. Existing pointers remain valid through growth —
// chunks are never moved or reallocated. On Reset, if growth occurred, all
// chunks collapse into a single fresh chunk sized to the High-Water Mark,
// making subsequent reuse cache-friendly.
//
// canAlloc always returns true; growth is bounded only by OS-level memory.
type GrowingArena interface {
	Arena

	// ChunkCount returns the number of chunks currently allocated.
	ChunkCount() int

	// HighWaterMark returns the peak total bytes allocated across all chunks
	// during this arena's current lifetime.
	HighWaterMark() int64
}

// GrowingArenaOption configures a GrowingArena at construction time.
type GrowingArenaOption func(*growingArena)

// WithMaxCollapseSize clamps the collapse target on Reset so a pathological
// large allocation does not permanently inflate every pooled arena. If the
// high-water mark exceeds cap, Reset targets cap instead; the next cycle that
// exceeds cap will grow again.
//
// A cap of 0 (the default) means unlimited — collapse always targets the full
// high-water mark.
func WithMaxCollapseSize(cap int64) GrowingArenaOption {
	return func(a *growingArena) {
		a.maxCollapseSize = cap
	}
}

// chunk is a fixed-size contiguous byte buffer; the unit of growth.
type chunk struct {
	buf    []byte
	cursor int
}

type growingArena struct {
	chunks          []chunk
	chunkSize       int64
	maxCollapseSize int64 // 0 = unlimited
	highWaterMark   int64
	totalBytes      int64
	refs            []any
	managed         map[int]any
	nextPinID       int
}

// active returns a pointer to the current chunk receiving new allocations.
func (a *growingArena) active() *chunk {
	return &a.chunks[len(a.chunks)-1]
}

// NewGrowingArena returns a new growing arena with an initial chunk of
// chunkSize bytes. The first chunk is allocated eagerly.
func NewGrowingArena(chunkSize int64, opts ...GrowingArenaOption) GrowingArena {
	a := &growingArena{
		chunkSize: chunkSize,
	}
	for _, o := range opts {
		o(a)
	}
	a.chunks = []chunk{{buf: make([]byte, chunkSize)}}
	a.totalBytes = chunkSize
	a.highWaterMark = chunkSize
	return a
}

// DestroyGrowingArena zeroes all chunks, releases all pins, and sets the
// GrowingArena variable to nil.
//
// Existing pointers into the arena's chunks still reference the underlying
// buffers (they now read as zeroes). The GC keeps the buffers alive until all
// such pointers go out of scope. Nil out arena-derived pointers when they are
// no longer needed.
func DestroyGrowingArena(arena *GrowingArena) {
	(*arena).(*growingArena).destroy()
	*arena = nil
}

// canAlloc always returns true; a GrowingArena grows on demand.
// align is always a power of 2 (guaranteed by unsafe.Alignof).
func (a *growingArena) canAlloc(size, align int) bool {
	return true
}

func (a *growingArena) alloc(size, align int) unsafe.Pointer {
	ac := a.active()
	padding := (align - ac.cursor%align) % align

	if ac.cursor+padding+size <= len(ac.buf) {
		// Fast path: fits in the active chunk. totalBytes is unchanged here, so
		// highWaterMark cannot grow — it only advances when a new chunk is added.
		start := ac.cursor + padding
		ptr := unsafe.Pointer(&ac.buf[start])
		ac.cursor = start + size
		clear(ac.buf[start : start+size])
		return ptr
	}

	// Does not fit — add a new chunk.
	var newBuf []byte
	if int64(size) > a.chunkSize {
		// Oversized request: one-off chunk rounded up to 8-byte boundary.
		rounded := int64((size + 7) &^ 7)
		newBuf = make([]byte, rounded)
		a.totalBytes += rounded
	} else {
		newBuf = make([]byte, a.chunkSize)
		a.totalBytes += a.chunkSize
	}

	a.chunks = append(a.chunks, chunk{buf: newBuf})
	if a.totalBytes > a.highWaterMark {
		a.highWaterMark = a.totalBytes
	}

	ac = a.active()
	// Alignment padding in the fresh chunk.
	padding = (align - ac.cursor%align) % align
	start := ac.cursor + padding
	ptr := unsafe.Pointer(&ac.buf[start])
	ac.cursor = start + size
	clear(ac.buf[start : start+size])
	return ptr
}

func (a *growingArena) free(ptr unsafe.Pointer) error {
	return nil
}

// growInPlace extends the tail allocation of the active chunk. An allocation
// is the tail iff it ends exactly at the active chunk's cursor; growth then
// only needs the extra bytes to fit before the end of that chunk. Older chunks
// are never reallocated, so allocations not in the active chunk cannot grow in
// place.
func (a *growingArena) growInPlace(ptr unsafe.Pointer, oldSize, newSize, align int) bool {
	ac := a.active()
	if len(ac.buf) == 0 {
		return false
	}
	// Stray pointers before buf[0] wrap to a large positive value on
	// unsigned arithmetic, caught by the >= len check.
	offset := int(uintptr(ptr) - uintptr(unsafe.Pointer(&ac.buf[0])))
	if offset >= len(ac.buf) || offset+oldSize != ac.cursor {
		return false
	}
	end := offset + newSize
	if end > len(ac.buf) {
		return false
	}
	clear(ac.buf[ac.cursor:end])
	ac.cursor = end
	return true
}

func (a *growingArena) pin(value any) {
	a.refs = append(a.refs, value)
}

func (a *growingArena) pinManaged(value any) int {
	id := a.nextPinID
	a.nextPinID++
	if a.managed == nil {
		a.managed = make(map[int]any)
	}
	a.managed[id] = value
	return id
}

func (a *growingArena) unpin(id int) {
	delete(a.managed, id)
}

// Reset resets the arena for reuse.
//
// If the arena never grew (still on chunk 0), Reset is a pure cursor rewind —
// the underlying buffer is kept and no allocation occurs.
//
// If the arena grew (multiple chunks), all chunks are dropped and replaced by
// a single fresh chunk. The new chunk is sized to the High-Water Mark, capped
// by WithMaxCollapseSize if set, and never smaller than chunkSize.
//
// All pointers obtained from the arena before Reset are invalidated.
// Dereferencing them leads to undefined behavior. All bulk pins are released.
func (a *growingArena) Reset() {
	if len(a.chunks) == 1 {
		// Never grew — rewind cursor only, keep the existing buffer.
		a.chunks[0].cursor = 0
		a.refs = a.refs[:0]
		clear(a.managed)
		return
	}

	// Grew — collapse all chunks into one.
	target := a.highWaterMark
	if a.maxCollapseSize > 0 && target > a.maxCollapseSize {
		target = a.maxCollapseSize
	}
	if target < a.chunkSize {
		target = a.chunkSize
	}

	// Drop old chunks (let GC reclaim them) and install a fresh single chunk.
	a.chunks = []chunk{{buf: make([]byte, target)}}
	a.totalBytes = target
	a.highWaterMark = target

	a.refs = a.refs[:0]
	clear(a.managed)
}

func (a *growingArena) destroy() {
	for i := range a.chunks {
		clear(a.chunks[i].buf)
		a.chunks[i].buf = nil
	}
	a.chunks = nil
	a.refs = nil
	a.managed = nil
}

func (a *growingArena) ChunkCount() int {
	return len(a.chunks)
}

func (a *growingArena) HighWaterMark() int64 {
	return a.highWaterMark
}
