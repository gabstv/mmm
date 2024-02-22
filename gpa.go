package mmm

import (
	"unsafe"
)

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
	remainder := b.cursor % align
	return b.cursor+size+remainder <= len(b.buf)
}

func (b *gpabucket) alloc(size, align int) unsafe.Pointer {
	remainder := b.cursor % align
	ptr := unsafe.Pointer(&b.buf[b.cursor+remainder])
	b.cursor += size + remainder
	b.allocations++
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
	// GPA can always allocate (if the OS doen't run out of memory)
	return true
}

func (a *generalPurposeAllocator) alloc(size, align int) unsafe.Pointer {
	bucket := a.getBucket(size, align)
	if bucket == nil {
		bucket = a.makeBucket(size)
	}

	return bucket.alloc(size, align)
}

func (a *generalPurposeAllocator) free(ptr unsafe.Pointer) error {
	for i := range a.buckets {
		if !a.buckets[i].hasPtr(ptr) {
			continue
		}

		// this is the bucket
		if a.buckets[i].free(ptr) {
			// bucket is empty, remove it
			a.buckets = append(a.buckets[:i], a.buckets[i+1:]...)
		}
		return nil
	}

	return ErrNotFound
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

// from https://go.dev/src/runtime/slice.go
type slice struct {
	array unsafe.Pointer
	len   int
	cap   int
}

func (a *generalPurposeAllocator) NewArena(size int) Arena {
	sz1 := unsafe.Sizeof(arenaAllocator{})
	sz2 := uintptr(size)
	arenaRoot := a.alloc(int(sz1)+int(sz2), 8)
	slc0 := (*slice)(arenaRoot)
	slc0.cap = size
	slc0.len = size
	slc0.array = unsafe.Pointer(uintptr(arenaRoot) + sz1)

	arena := (*arenaAllocator)(arenaRoot)
	arena.cursor = 0
	arena.parent = a

	return arena
}

func NewGeneralPurposeAllocator(bucketSize int) GeneralPurposeAllocator {
	return &generalPurposeAllocator{
		bucketSize: bucketSize,
	}
}
