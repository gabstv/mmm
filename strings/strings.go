package mstrings

import (
	"unsafe"

	"github.com/gabstv/mmm"
)

// String is a pointer to an arena-allocated string header. The byte data
// lives immediately after the header in contiguous memory.
//
// Memory layout: [len:4][cap:4][data:cap][0x00]
//
// No pointers, no GC pinning needed. Always null-terminated for C compat.
type String = *header

type header struct {
	len uint32
	cap uint32
}

const headerSize = unsafe.Sizeof(header{})
const headerAlign = unsafe.Alignof(header{})

func dataPtr(s String) unsafe.Pointer {
	return unsafe.Add(unsafe.Pointer(s), headerSize)
}

func dataSlice(s String) []byte {
	return unsafe.Slice((*byte)(dataPtr(s)), s.cap+1)
}

// AlignedCap returns the smallest capacity >= cap such that the total
// allocation (header + cap + null terminator) is a multiple of the
// header alignment, eliminating padding waste in the allocator.
func AlignedCap(cap int) int {
	return (cap+int(headerAlign))&^(int(headerAlign)-1) - 1
}

// New allocates a String with the given capacity. The length is 0.
// Returns nil if the allocator cannot satisfy the request.
func New(a mmm.Allocator, cap int) String {
	total := int(headerSize) + cap + 1 // +1 for null terminator
	ptr := mmm.RawAlloc(a, total, int(headerAlign))
	if ptr == nil {
		return nil
	}
	s := (String)(ptr)
	s.len = 0
	s.cap = uint32(cap)
	return s
}

// From allocates a String and copies the contents of a Go string into it.
// Capacity equals the string length. Returns nil if allocation fails.
func From(a mmm.Allocator, str string) String {
	s := New(a, len(str))
	if s == nil {
		return nil
	}
	buf := dataSlice(s)
	copy(buf, str)
	buf[len(str)] = 0
	s.len = uint32(len(str))
	return s
}

// FromBytes allocates a String and copies the contents of a byte slice into it.
// Capacity equals the slice length. Returns nil if allocation fails.
func FromBytes(a mmm.Allocator, b []byte) String {
	s := New(a, len(b))
	if s == nil {
		return nil
	}
	buf := dataSlice(s)
	copy(buf, b)
	buf[len(b)] = 0
	s.len = uint32(len(b))
	return s
}

// Len returns the current byte length of the string (excluding the null terminator).
func Len(s String) int {
	return int(s.len)
}

// Cap returns the capacity in bytes (maximum length before reallocation would be needed).
func Cap(s String) int {
	return int(s.cap)
}

// GoString returns the contents as a Go string without copying.
// The returned string is valid only while the underlying arena is alive.
func GoString(s String) string {
	return unsafe.String((*byte)(dataPtr(s)), s.len)
}

// Bytes returns the contents as a byte slice without copying.
// The returned slice is valid only while the underlying arena is alive.
func Bytes(s String) []byte {
	return unsafe.Slice((*byte)(dataPtr(s)), s.len)
}

// CString returns a pointer to the null-terminated byte data, suitable
// for passing to C functions via cgo.
func CString(s String) *byte {
	return (*byte)(dataPtr(s))
}

// Set overwrites the string content. Returns false if data exceeds capacity.
func Set(s String, data string) bool {
	if len(data) > int(s.cap) {
		return false
	}
	buf := dataSlice(s)
	copy(buf, data)
	buf[len(data)] = 0
	s.len = uint32(len(data))
	return true
}

// SetBytes overwrites the string content from a byte slice.
// Returns false if data exceeds capacity.
func SetBytes(s String, data []byte) bool {
	if len(data) > int(s.cap) {
		return false
	}
	buf := dataSlice(s)
	copy(buf, data)
	buf[len(data)] = 0
	s.len = uint32(len(data))
	return true
}

// Append appends data to the string. Returns false if the result would exceed capacity.
func Append(s String, data string) bool {
	newLen := int(s.len) + len(data)
	if newLen > int(s.cap) {
		return false
	}
	buf := dataSlice(s)
	copy(buf[s.len:], data)
	buf[newLen] = 0
	s.len = uint32(newLen)
	return true
}

// AppendBytes appends a byte slice to the string.
// Returns false if the result would exceed capacity.
func AppendBytes(s String, data []byte) bool {
	newLen := int(s.len) + len(data)
	if newLen > int(s.cap) {
		return false
	}
	buf := dataSlice(s)
	copy(buf[s.len:], data)
	buf[newLen] = 0
	s.len = uint32(newLen)
	return true
}

// Free releases the string's allocation from the allocator.
func Free(a mmm.Allocator, s String) error {
	return mmm.RawFree(a, unsafe.Pointer(s))
}

// Equal returns true if two Strings have identical content.
func Equal(a, b String) bool {
	if a.len != b.len {
		return false
	}
	ab := unsafe.Slice((*byte)(dataPtr(a)), a.len)
	bb := unsafe.Slice((*byte)(dataPtr(b)), b.len)
	for i := range ab {
		if ab[i] != bb[i] {
			return false
		}
	}
	return true
}

// EqualString returns true if the String has identical content to a Go string.
func EqualString(s String, gs string) bool {
	if int(s.len) != len(gs) {
		return false
	}
	sb := unsafe.Slice((*byte)(dataPtr(s)), s.len)
	for i := range sb {
		if sb[i] != gs[i] {
			return false
		}
	}
	return true
}
