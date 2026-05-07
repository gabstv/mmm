package mmm

import "unsafe"

// Allocator is the interface that all memory allocators must implement.
type Allocator interface {
	alloc(size, align int) unsafe.Pointer
	canAlloc(size, align int) bool
	free(ptr unsafe.Pointer) error
	pin(value any)
	pinManaged(value any) int
	unpin(id int)
}

// PinHandle is a handle to a managed pin that can be individually released.
// Use with PinManaged for allocations with different lifetimes (typical in
// GeneralPurposeAllocator usage).
type PinHandle struct {
	allocator Allocator
	id        int
}

// Unpin releases this pin, allowing the GC to collect the pinned value
// if no other references exist. Safe to call multiple times.
func (h *PinHandle) Unpin() {
	if h.allocator != nil {
		h.allocator.unpin(h.id)
		h.allocator = nil
	}
}

// TryAlloc allocates a value of type T from the allocator, returning an error
// if the allocator is out of memory. The returned memory is zeroed.
//
// WARNING: if T contains pointer-bearing fields (string, slice, map, chan,
// func, interface, or pointer fields), you MUST use Pin or PinManaged to
// keep those values visible to the garbage collector. Without pinning, the
// GC may collect the targets, leaving dangling references.
func TryAlloc[T any](allocator Allocator) (*T, error) {
	var zt T
	sz := unsafe.Sizeof(zt)
	az := unsafe.Alignof(zt)

	if !allocator.canAlloc(int(sz), int(az)) {
		return nil, ErrOutOfMemory
	}

	pp := allocator.alloc(int(sz), int(az))

	return (*T)(pp), nil
}

// Alloc allocates a value of type T from the allocator.
// Panics if the allocator is out of memory. Use TryAlloc for error handling.
// The returned memory is zeroed.
//
// WARNING: if T contains pointer-bearing fields (string, slice, map, chan,
// func, interface, or pointer fields), you MUST use Pin or PinManaged to
// keep those values visible to the garbage collector. Without pinning, the
// GC may collect the targets, leaving dangling references.
func Alloc[T any](allocator Allocator) *T {
	var zt T
	sz := unsafe.Sizeof(zt)
	az := unsafe.Alignof(zt)

	pp := allocator.alloc(int(sz), int(az))
	if pp == nil {
		panic("mmm: out of memory")
	}

	return (*T)(pp)
}

// Free frees a value previously allocated by the allocator and sets the
// pointer to nil. For arena allocators, this is a no-op (memory is only
// reclaimed on Reset or Destroy).
//
// Free does NOT release managed pins (PinManaged). If you pinned values
// associated with this allocation, you must call PinHandle.Unpin()
// separately to avoid keeping unnecessary GC references alive.
func Free[T any](allocator Allocator, ptr **T) error {
	if err := allocator.free(unsafe.Pointer(*ptr)); err != nil {
		return err
	}

	*ptr = nil

	return nil
}

// Pin keeps a reference to value visible to the garbage collector.
// All bulk pins are released on Arena.Reset() or DestroyArena().
//
// Use this when storing pointer-bearing values (strings, slices, maps,
// interfaces, function values, or structs containing any of these) in
// arena-allocated memory. Without Pin, the GC cannot see references
// stored inside the arena's []byte buffer and may collect the targets.
//
// For GeneralPurposeAllocator, prefer PinManaged so pins can be released
// individually when allocations are freed.
//
// Returns the value unchanged for ergonomic inline use:
//
//	s := Alloc[MyStruct](arena)
//	s.Name = Pin(arena, buildString())
func Pin[T any](allocator Allocator, value T) T {
	allocator.pin(value)
	return value
}

// PinManaged keeps a reference visible to the GC and returns a handle
// for individual release. Preferred over Pin for GeneralPurposeAllocator
// where allocations have different lifetimes.
//
// Call PinHandle.Unpin() when the pinned value is no longer stored in
// arena memory (typically before or after Free).
//
//	s := Alloc[MyStruct](gpa)
//	s.Name, h := PinManaged(gpa, buildString())
//	// ... use s ...
//	h.Unpin()
//	Free(gpa, &s)
func PinManaged[T any](allocator Allocator, value T) (T, PinHandle) {
	id := allocator.pinManaged(value)
	return value, PinHandle{allocator: allocator, id: id}
}
