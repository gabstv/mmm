// Package mring provides a fixed-capacity ring buffer allocated in mmm arena
// memory. It is designed for game-dev scenarios where a bounded history of
// recent values is needed with zero per-frame allocations: input buffering
// for combo detection, position trails for motion blur, event logs, audio
// sample buffers, and network jitter buffers.
//
// Per-element throughput is lower than native Go slices because the compiler
// cannot optimize generic unsafe pointer arithmetic as aggressively as
// built-in append. However, typical usage pushes one element per frame, not
// thousands in a tight loop — at that scale the overhead is invisible, and
// the ring buffer provides zero GC pressure with fixed memory for the
// lifetime of the game.
package mring

import (
	"encoding/json"
	"errors"
	"math/bits"
	"unsafe"

	"github.com/gabstv/mmm"
)

// Ring is a pointer to an arena-allocated ring buffer header.
// Element data lives after the header in contiguous memory.
//
// Memory layout: [head:4][len:4][cap:4][mask:4][elemSize:4][dataOffset:4][element data]
//
// Capacity is always rounded up to the next power of 2 so that index
// wrapping uses a bitmask (AND) instead of modular division.
// Element size and data offset are cached in the header at creation time.
//
// Elements are logically ordered from oldest (index 0) to newest (index Len-1).
// When full, Push overwrites the oldest element.
type Ring = *ringHeader

type ringHeader struct {
	head       uint32
	len        uint32
	cap        uint32
	mask       uint32
	elemSize   uint32
	dataOffset uint32
}

const headerSize = unsafe.Sizeof(ringHeader{})
const headerAlign = unsafe.Alignof(ringHeader{})

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

func nextPowerOf2(n int) int {
	if n <= 1 {
		return 1
	}
	return 1 << bits.Len(uint(n-1))
}

// New allocates a Ring with at least the given capacity.
// The actual capacity is rounded up to the next power of 2.
// Returns nil if the allocator cannot satisfy the request.
func New[T any](a mmm.Allocator, cap int) Ring {
	cap = nextPowerOf2(cap)
	es := elemSize[T]()
	do := computeDataOffset[T]()
	total := int(do) + cap*int(es)
	ptr := mmm.RawAlloc(a, total, allocAlign[T]())
	if ptr == nil {
		return nil
	}
	r := (Ring)(ptr)
	r.head = 0
	r.len = 0
	r.cap = uint32(cap)
	r.mask = uint32(cap - 1)
	r.elemSize = uint32(es)
	r.dataOffset = uint32(do)
	return r
}

// Len returns the number of elements in the ring buffer.
func Len(r Ring) int {
	return int(r.len)
}

// Cap returns the capacity of the ring buffer (always a power of 2).
func Cap(r Ring) int {
	return int(r.cap)
}

// Full returns true if the ring buffer is at capacity.
func Full(r Ring) bool {
	return r.len == r.cap
}

// Empty returns true if the ring buffer has no elements.
func Empty(r Ring) bool {
	return r.len == 0
}

// Push adds an element to the back. If the buffer is full, the oldest
// element is overwritten and the head advances.
func Push[T any](r Ring, v T) {
	tail := (r.head + r.len) & r.mask
	*(*T)(unsafe.Add(unsafe.Pointer(r), uintptr(r.dataOffset)+uintptr(tail)*uintptr(r.elemSize))) = v
	if r.len < r.cap {
		r.len++
	} else {
		r.head = (r.head + 1) & r.mask
	}
}

// TryPush adds an element to the back without overwriting.
// Returns false if the buffer is full.
func TryPush[T any](r Ring, v T) bool {
	if r.len >= r.cap {
		return false
	}
	tail := (r.head + r.len) & r.mask
	*(*T)(unsafe.Add(unsafe.Pointer(r), uintptr(r.dataOffset)+uintptr(tail)*uintptr(r.elemSize))) = v
	r.len++
	return true
}

// Pop removes and returns the oldest element.
// Returns the zero value and false if empty.
func Pop[T any](r Ring) (T, bool) {
	if r.len == 0 {
		var zero T
		return zero, false
	}
	v := *(*T)(unsafe.Add(unsafe.Pointer(r), uintptr(r.dataOffset)+uintptr(r.head)*uintptr(r.elemSize)))
	r.head = (r.head + 1) & r.mask
	r.len--
	return v, true
}

// Peek returns the oldest element without removing it.
// Returns the zero value and false if empty.
func Peek[T any](r Ring) (T, bool) {
	if r.len == 0 {
		var zero T
		return zero, false
	}
	v := *(*T)(unsafe.Add(unsafe.Pointer(r), uintptr(r.dataOffset)+uintptr(r.head)*uintptr(r.elemSize)))
	return v, true
}

// PeekBack returns the newest element without removing it.
// Returns the zero value and false if empty.
func PeekBack[T any](r Ring) (T, bool) {
	if r.len == 0 {
		var zero T
		return zero, false
	}
	tail := (r.head + r.len - 1) & r.mask
	v := *(*T)(unsafe.Add(unsafe.Pointer(r), uintptr(r.dataOffset)+uintptr(tail)*uintptr(r.elemSize)))
	return v, true
}

// Get returns a pointer to the element at logical index i,
// where 0 is the oldest and Len-1 is the newest.
// Panics if i is out of bounds.
func Get[T any](r Ring, i int) *T {
	if i < 0 || i >= int(r.len) {
		panic("mring: index out of range")
	}
	physical := (r.head + uint32(i)) & r.mask
	return (*T)(unsafe.Add(unsafe.Pointer(r), uintptr(r.dataOffset)+uintptr(physical)*uintptr(r.elemSize)))
}

// Set writes a value at logical index i.
// Panics if i is out of bounds.
func Set[T any](r Ring, i int, v T) {
	if i < 0 || i >= int(r.len) {
		panic("mring: index out of range")
	}
	physical := (r.head + uint32(i)) & r.mask
	*(*T)(unsafe.Add(unsafe.Pointer(r), uintptr(r.dataOffset)+uintptr(physical)*uintptr(r.elemSize))) = v
}

// Clear resets the ring buffer to empty. Does not zero memory.
func Clear(r Ring) {
	r.head = 0
	r.len = 0
}

// MarshalJSON serializes the ring buffer as a JSON array in logical order
// (oldest to newest).
func MarshalJSON[T any](r Ring) ([]byte, error) {
	elems := make([]T, r.len)
	for i := range int(r.len) {
		elems[i] = *Get[T](r, i)
	}
	return json.Marshal(elems)
}

// UnmarshalJSON deserializes a JSON array into the ring buffer.
// Returns an error if the array length exceeds capacity.
// The ring is cleared before elements are inserted.
func UnmarshalJSON[T any](r Ring, data []byte) error {
	var elems []T
	if err := json.Unmarshal(data, &elems); err != nil {
		return err
	}
	if len(elems) > int(r.cap) {
		return errors.New("mring: JSON array exceeds Ring capacity")
	}
	Clear(r)
	for _, v := range elems {
		Push[T](r, v)
	}
	return nil
}

// Free releases the ring buffer's allocation from the allocator.
func Free(a mmm.Allocator, r Ring) error {
	return mmm.RawFree(a, unsafe.Pointer(r))
}
