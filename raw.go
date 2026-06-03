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

// reallocator is implemented by allocators that can grow the most recent
// (tail) allocation in place, avoiding a copy. The optional interface is
// probed by RawRealloc via type assertion.
type reallocator interface {
	// growInPlace attempts to extend the allocation at ptr from oldSize to
	// newSize bytes without moving it. It returns true only if ptr is the
	// tail allocation and there is room to grow.
	growInPlace(ptr unsafe.Pointer, oldSize, newSize, align int) bool
}

// RawRealloc grows the allocation at ptr to newSize bytes, returning a pointer
// to a block of at least newSize bytes that preserves the existing contents.
//
// If ptr is nil, RawRealloc behaves like RawAlloc(a, newSize, align).
//
// Behavior:
//   - If the allocator can grow the allocation in place (it is the most recent
//     allocation and the backing buffer has room), the same pointer is returned
//     with no copy. The Arena, GrowingArena, and GeneralPurposeAllocator
//     implementations all support in-place growth of their tail allocation.
//   - Otherwise RawRealloc allocates a fresh block of newSize bytes, copies the
//     first oldSize bytes from the old block, frees the old block (so it is
//     reclaimed rather than orphaned — a no-op for bump arenas, real reuse for
//     GeneralPurposeAllocator), and returns the new pointer.
//
// Returns nil (without freeing the old block) if the allocator cannot satisfy
// newSize bytes. When newSize <= oldSize the original pointer is returned
// unchanged (no-op); only growth performs work.
//
// The caller must pass the exact oldSize the allocation was created with so the
// fallback copy preserves all live bytes.
func RawRealloc(a Allocator, ptr unsafe.Pointer, oldSize, newSize, align int) unsafe.Pointer {
	if ptr == nil {
		return RawAlloc(a, newSize, align)
	}
	if newSize <= oldSize {
		return ptr
	}

	// Fast path: try in-place growth of the tail allocation.
	if ra, ok := a.(reallocator); ok {
		if ra.growInPlace(ptr, oldSize, newSize, align) {
			return ptr
		}
	}

	// Fallback: alloc-new + copy + free-old (reclaims the old block instead
	// of orphaning it).
	if !a.canAlloc(newSize, align) {
		return nil
	}
	np := a.alloc(newSize, align)
	if np == nil {
		return nil
	}
	copy(unsafe.Slice((*byte)(np), oldSize), unsafe.Slice((*byte)(ptr), oldSize))
	_ = a.free(ptr)
	return np
}
