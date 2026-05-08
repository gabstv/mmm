package mslices

import (
	"unsafe"

	"github.com/gabstv/mmm"
)

// Slice is a pointer to an arena-allocated slice header. The element data
// lives after the header in contiguous memory (with alignment padding if
// needed).
//
// Memory layout: [len:4][cap:4][elemSize:4][dataOffset:4][element data: cap * sizeof(T)]
//
// Element size and data offset are cached in the header at creation time,
// so hot-path operations (Get, Set, Append) avoid recomputing them from
// the type parameter on every call.
//
// For value types without pointers, no GC pinning is needed.
type Slice = *header

type header struct {
	len        uint32
	cap        uint32
	elemSize   uint32
	dataOffset uint32
}

const headerSize = unsafe.Sizeof(header{})
const headerAlign = unsafe.Alignof(header{})

func elemSize[T any]() uintptr {
	var zt T
	return unsafe.Sizeof(zt)
}

func elemAlign[T any]() uintptr {
	var zt T
	return unsafe.Alignof(zt)
}

func computeDataOffset[T any]() uintptr {
	a := elemAlign[T]()
	return (headerSize + a - 1) &^ (a - 1)
}

func allocAlign[T any]() int {
	a := int(elemAlign[T]())
	if a > int(headerAlign) {
		return a
	}
	return int(headerAlign)
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// AlignedCap returns the smallest capacity >= cap such that the total
// allocation (header + padding + cap*sizeof(T)) is a multiple of the
// required alignment, eliminating padding waste in the allocator.
func AlignedCap[T any](cap int) int {
	es := int(elemSize[T]())
	aa := allocAlign[T]()
	step := aa / gcd(es, aa)
	return ((cap + step - 1) / step) * step
}

// New allocates a Slice with the given element capacity. Length is 0.
// Returns nil if the allocator cannot satisfy the request.
func New[T any](a mmm.Allocator, cap int) Slice {
	es := elemSize[T]()
	do := computeDataOffset[T]()
	total := int(do) + cap*int(es)
	ptr := mmm.RawAlloc(a, total, allocAlign[T]())
	if ptr == nil {
		return nil
	}
	s := (Slice)(ptr)
	s.len = 0
	s.cap = uint32(cap)
	s.elemSize = uint32(es)
	s.dataOffset = uint32(do)
	return s
}

// From allocates a Slice and copies the elements from a Go slice.
// Capacity equals the slice length. Returns nil if allocation fails.
func From[T any](a mmm.Allocator, data []T) Slice {
	s := New[T](a, len(data))
	if s == nil {
		return nil
	}
	dst := unsafe.Slice((*T)(unsafe.Add(unsafe.Pointer(s), uintptr(s.dataOffset))), len(data))
	copy(dst, data)
	s.len = uint32(len(data))
	return s
}

// Len returns the number of elements in the slice.
func Len(s Slice) int {
	return int(s.len)
}

// Cap returns the element capacity of the slice.
func Cap(s Slice) int {
	return int(s.cap)
}

// Get returns a pointer to the element at index i.
// The pointer is valid only while the underlying allocator is alive.
// Panics if i is out of bounds.
func Get[T any](s Slice, i int) *T {
	if i < 0 || i >= int(s.len) {
		panic("mslices: index out of range")
	}
	return (*T)(unsafe.Add(unsafe.Pointer(s), uintptr(s.dataOffset)+uintptr(i)*uintptr(s.elemSize)))
}

// Set writes a value to the element at index i.
// Panics if i is out of bounds.
func Set[T any](s Slice, i int, v T) {
	if i < 0 || i >= int(s.len) {
		panic("mslices: index out of range")
	}
	*(*T)(unsafe.Add(unsafe.Pointer(s), uintptr(s.dataOffset)+uintptr(i)*uintptr(s.elemSize))) = v
}

// Append adds an element to the slice. Returns false if the slice is at capacity.
func Append[T any](s Slice, v T) bool {
	if s.len >= s.cap {
		return false
	}
	*(*T)(unsafe.Add(unsafe.Pointer(s), uintptr(s.dataOffset)+uintptr(s.len)*uintptr(s.elemSize))) = v
	s.len++
	return true
}

// AppendSlice appends all elements from a Go slice.
// Returns false if the result would exceed capacity.
func AppendSlice[T any](s Slice, data []T) bool {
	newLen := int(s.len) + len(data)
	if newLen > int(s.cap) {
		return false
	}
	dst := unsafe.Slice((*T)(unsafe.Add(unsafe.Pointer(s), uintptr(s.dataOffset)+uintptr(s.len)*uintptr(s.elemSize))), len(data))
	copy(dst, data)
	s.len = uint32(newLen)
	return true
}

// GoSlice returns the contents as a Go slice without copying.
// The returned slice is valid only while the underlying allocator is alive.
func GoSlice[T any](s Slice) []T {
	return unsafe.Slice((*T)(unsafe.Add(unsafe.Pointer(s), uintptr(s.dataOffset))), s.len)
}

// Ptr returns a pointer to the first element, suitable for passing to
// C functions via cgo. Returns nil if the slice is empty.
//
// The pointer points into arena memory (a Go-allocated []byte buffer).
// For cgo, you may need to disable the pointer checker:
//
//	#cgo CFLAGS: -DCGO_CHECK=0
//
// or build with GOEXPERIMENT=nocheckcgopointer.
func Ptr[T any](s Slice) *T {
	if s.len == 0 {
		return nil
	}
	return (*T)(unsafe.Add(unsafe.Pointer(s), uintptr(s.dataOffset)))
}

// Clear sets the length to 0. Does not zero the underlying memory.
func Clear(s Slice) {
	s.len = 0
}

// Truncate sets the length to n. Panics if n > current length.
func Truncate(s Slice, n int) {
	if n < 0 || n > int(s.len) {
		panic("mslices: truncate out of range")
	}
	s.len = uint32(n)
}

// Free releases the slice's allocation from the allocator.
func Free(a mmm.Allocator, s Slice) error {
	return mmm.RawFree(a, unsafe.Pointer(s))
}
