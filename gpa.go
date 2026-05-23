package mmm

import (
	"cmp"
	"slices"
	"unsafe"
)

// GeneralPurposeAllocator is a bucket-based allocator that supports
// individual Free calls. Each bucket is a bump allocator; when all
// allocations in a bucket are freed, the bucket is reclaimed.
//
// Freed memory within a bucket is tracked in a free list and reused for
// subsequent allocations. Adjacent free regions are coalesced on insert.
// When the tail of the bump region is freed, the cursor moves back to
// allow bump allocation of that space again.
type GeneralPurposeAllocator interface {
	Allocator
	Size() int64
	Count() int
	free(ptr unsafe.Pointer) error
	NewArena(size int) Arena
}

type generalPurposeAllocator struct {
	bucketSize int
	buckets    []gpabucket
	refs       []any
	managed    map[int]any
	nextPinID  int
}

type gpabucket struct {
	buf         []byte
	cursor      int
	allocations int
	allocSizes  map[int]int // offset-in-buf → allocation size (lazily initialized)
	freeRegions []region    // sorted by pos, coalesced on insert
}

type region struct {
	pos  int
	size int
}

func (b *gpabucket) canAlloc(size, align int) bool {
	// Check bump space first
	padding := (align - b.cursor%align) % align
	if b.cursor+padding+size <= len(b.buf) {
		return true
	}
	// Check free list for a fitting region
	for _, r := range b.freeRegions {
		padding := (align - r.pos%align) % align
		alignedStart := r.pos + padding
		_ = alignedStart
		if padding+size <= r.size {
			return true
		}
	}
	return false
}

// allocFromFreeList tries to satisfy an allocation from the free list.
// Returns nil if no suitable region is found.
func (b *gpabucket) allocFromFreeList(size, align int) unsafe.Pointer {
	for i, r := range b.freeRegions {
		padding := (align - r.pos%align) % align
		alignedStart := r.pos + padding

		if padding+size > r.size {
			continue
		}

		// This region fits — remove it from the list
		b.freeRegions = slices.Delete(b.freeRegions, i, i+1)

		// Return the padding fragment before the allocation to the free list
		// (only if it's large enough to avoid extreme fragmentation)
		if padding >= 8 {
			b.insertFreeRegion(region{pos: r.pos, size: padding})
		}

		// Return any leftover after the allocation to the free list
		remaining := r.size - padding - size
		if remaining >= 8 {
			b.insertFreeRegion(region{pos: alignedStart + size, size: remaining})
		}

		// Record size, update counter, zero memory, return pointer
		if b.allocSizes == nil {
			b.allocSizes = make(map[int]int)
		}
		b.allocSizes[alignedStart] = size
		b.allocations++
		clear(b.buf[alignedStart : alignedStart+size])
		return unsafe.Pointer(&b.buf[alignedStart])
	}
	return nil
}

func (b *gpabucket) alloc(size, align int) unsafe.Pointer {
	// Try free list first
	if ptr := b.allocFromFreeList(size, align); ptr != nil {
		return ptr
	}

	// Fall back to bump allocation
	padding := (align - b.cursor%align) % align
	start := b.cursor + padding
	ptr := unsafe.Pointer(&b.buf[start])
	b.cursor = start + size
	b.allocations++

	// Record allocation size (lazily init map)
	if b.allocSizes == nil {
		b.allocSizes = make(map[int]int)
	}
	b.allocSizes[start] = size

	clear(b.buf[start : start+size])
	return ptr
}

func (b *gpabucket) hasPtr(ptr unsafe.Pointer) bool {
	if len(b.buf) == 0 {
		return false
	}
	p0 := uintptr(unsafe.Pointer(&b.buf[0]))
	p := uintptr(ptr)
	return p >= p0 && p < p0+uintptr(len(b.buf))
}

func (b *gpabucket) free(ptr unsafe.Pointer) (bucketEmptied bool) {
	b.allocations--

	if b.allocations == 0 {
		b.cursor = 0
		b.allocSizes = nil
		b.freeRegions = b.freeRegions[:0]
		return true
	}

	// Compute offset into the buffer
	offset := int(uintptr(ptr) - uintptr(unsafe.Pointer(&b.buf[0])))

	// Look up size in allocSizes map
	size, ok := b.allocSizes[offset]
	if !ok {
		// Shouldn't happen, fall back gracefully (no reclamation)
		return false
	}
	delete(b.allocSizes, offset)

	// Insert the freed region into the free list (with coalescing)
	b.insertFreeRegion(region{pos: offset, size: size})

	// Shrink cursor if the freed region(s) extend to the tail
	b.reclaimTail()

	return false
}

// insertFreeRegion inserts r into freeRegions (sorted by pos) and coalesces
// adjacent regions.
func (b *gpabucket) insertFreeRegion(r region) {
	// Binary search for the insertion point
	idx, _ := slices.BinarySearchFunc(b.freeRegions, r, func(x, y region) int {
		return cmp.Compare(x.pos, y.pos)
	})

	b.freeRegions = slices.Insert(b.freeRegions, idx, r)

	// Coalesce with next region if adjacent
	if idx+1 < len(b.freeRegions) {
		next := b.freeRegions[idx+1]
		if b.freeRegions[idx].pos+b.freeRegions[idx].size == next.pos {
			b.freeRegions[idx].size += next.size
			b.freeRegions = slices.Delete(b.freeRegions, idx+1, idx+2)
		}
	}

	// Coalesce with previous region if adjacent
	if idx > 0 {
		prev := b.freeRegions[idx-1]
		if prev.pos+prev.size == b.freeRegions[idx].pos {
			b.freeRegions[idx-1].size += b.freeRegions[idx].size
			b.freeRegions = slices.Delete(b.freeRegions, idx, idx+1)
		}
	}
}

// reclaimTail moves the cursor back if the last free region extends to it,
// then recurses to handle the new tail.
func (b *gpabucket) reclaimTail() {
	if len(b.freeRegions) == 0 {
		return
	}
	last := b.freeRegions[len(b.freeRegions)-1]
	if last.pos+last.size == b.cursor {
		b.cursor = last.pos
		b.freeRegions = b.freeRegions[:len(b.freeRegions)-1]
		b.reclaimTail()
	}
}

func (b *generalPurposeAllocator) canAlloc(size int, align int) bool {
	return true
}

func (a *generalPurposeAllocator) alloc(size, align int) unsafe.Pointer {
	bucket := a.getBucket(size, align)
	if bucket == nil {
		bucket = a.makeBucket(size)
	}

	return bucket.alloc(size, align)
}

// Free releases an allocation from the GPA. When all allocations within
// a bucket are freed, the bucket itself is reclaimed.
//
// Free does NOT release managed pins (PinManaged). Call PinHandle.Unpin()
// separately to avoid keeping unnecessary GC references alive.
func (a *generalPurposeAllocator) free(ptr unsafe.Pointer) error {
	for i := range a.buckets {
		if !a.buckets[i].hasPtr(ptr) {
			continue
		}

		if a.buckets[i].free(ptr) {
			a.buckets = append(a.buckets[:i], a.buckets[i+1:]...)
		}
		return nil
	}

	return ErrNotFound
}

func (a *generalPurposeAllocator) pin(value any) {
	a.refs = append(a.refs, value)
}

func (a *generalPurposeAllocator) pinManaged(value any) int {
	id := a.nextPinID
	a.nextPinID++
	if a.managed == nil {
		a.managed = make(map[int]any)
	}
	a.managed[id] = value
	return id
}

func (a *generalPurposeAllocator) unpin(id int) {
	delete(a.managed, id)
}

func (a *generalPurposeAllocator) getBucket(freeSize, align int) *gpabucket {
	for i := range a.buckets {
		if a.buckets[i].canAlloc(freeSize, align) {
			return &a.buckets[i]
		}
	}

	return nil
}

func (a *generalPurposeAllocator) makeBucket(minsize int) *gpabucket {
	size := a.bucketSize
	if minsize > size {
		size = (minsize + 7) &^ 7
	}
	b := gpabucket{
		buf: make([]byte, size),
	}

	a.buckets = append(a.buckets, b)

	return &a.buckets[len(a.buckets)-1]
}

func (a *generalPurposeAllocator) Count() int {
	total := 0
	for _, b := range a.buckets {
		total += b.allocations
	}
	return total
}

func (a *generalPurposeAllocator) Size() int64 {
	var total int64
	for _, b := range a.buckets {
		total += int64(len(b.buf))
	}

	return total
}

// NewArena creates a sub-arena allocated from this GPA. The arena struct
// and its buffer are laid out contiguously in a single GPA allocation.
//
// Pin/PinManaged calls on the returned arena are delegated to the parent
// GPA, since the arena struct lives in GPA memory (invisible to the GC).
// Managed pin handles remain valid after the arena is destroyed.
func (a *generalPurposeAllocator) NewArena(size int) Arena {
	sz1 := unsafe.Sizeof(arenaAllocator{})
	arenaRoot := a.alloc(int(sz1)+size, 8)

	arena := (*arenaAllocator)(arenaRoot)
	bufStart := unsafe.Add(arenaRoot, int(sz1))
	arena.buf = unsafe.Slice((*byte)(bufStart), size)
	arena.cursor = 0
	arena.parent = a

	return arena
}

// NewGeneralPurposeAllocator returns a new GPA with the given default
// bucket size. Buckets larger than bucketSize are created automatically
// when an allocation exceeds the default.
func NewGeneralPurposeAllocator(bucketSize int) GeneralPurposeAllocator {
	return &generalPurposeAllocator{
		bucketSize: bucketSize,
	}
}
