package mmm

import "unsafe"

// RawAlloc allocates size bytes with the given alignment from the allocator.
// Returns nil if the allocator cannot satisfy the request.
//
// This is the low-level building block for variable-size types (strings,
// vectors) that need more than sizeof(T) bytes per allocation.
func RawAlloc(a Allocator, size, align int) unsafe.Pointer {
	if !a.canAlloc(size, align) {
		return nil
	}
	return a.alloc(size, align)
}

// RawFree frees a raw allocation previously obtained from RawAlloc.
func RawFree(a Allocator, ptr unsafe.Pointer) error {
	return a.free(ptr)
}
