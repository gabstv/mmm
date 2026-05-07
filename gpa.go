package mmm

import (
	"unsafe"
)

// GeneralPurposeAllocator is a bucket-based allocator that supports
// individual Free calls. Each bucket is a bump allocator; when all
// allocations in a bucket are freed, the bucket is reclaimed.
//
// Memory within a partially-freed bucket is NOT reused — the cursor
// only resets when the bucket's allocation count reaches zero. For
// workloads that interleave alloc/free of different sizes, memory
// will accumulate in buckets until they are fully emptied.
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
}

type region struct {
	pos  int
	size int
}

func (b *gpabucket) canAlloc(size, align int) bool {
	padding := (align - b.cursor%align) % align
	return b.cursor+padding+size <= len(b.buf)
}

func (b *gpabucket) alloc(size, align int) unsafe.Pointer {
	padding := (align - b.cursor%align) % align
	start := b.cursor + padding
	ptr := unsafe.Pointer(&b.buf[start])
	b.cursor = start + size
	b.allocations++
	clear(b.buf[start : start+size])
	return ptr
}

func (b *gpabucket) hasPtr(ptr unsafe.Pointer) bool {
	p00 := unsafe.Pointer(&b.buf[0])
	p0 := uintptr(p00)
	p := uintptr(ptr)
	pos := int(p - p0)
	return pos >= 0 && pos < len(b.buf)
}

func (b *gpabucket) free(ptr unsafe.Pointer) (bucketEmptied bool) {
	b.allocations--

	if b.allocations == 0 {
		b.cursor = 0
		return true
	}

	return false
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
		modulo := minsize % 8
		size = minsize + modulo
	}
	b := gpabucket{
		buf: make([]byte, size),
	}

	a.buckets = append(a.buckets, b)

	return &a.buckets[len(a.buckets)-1]
}

func (a *generalPurposeAllocator) Count() int {
	total := 0
	for i := range a.buckets {
		total += a.buckets[i].allocations
	}
	return total
}

func (a *generalPurposeAllocator) Size() int64 {
	var total int64
	for i := range a.buckets {
		total += int64(len(a.buckets[i].buf))
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
