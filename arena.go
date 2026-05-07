package mmm

import (
	"unsafe"
)

// Arena is a linear (bump) allocator. Allocations advance a cursor through
// a fixed-size buffer. Individual Free calls are no-ops — memory is reclaimed
// in bulk via Reset (reuse the buffer) or DestroyArena (release everything).
type Arena interface {
	Allocator

	// Reset resets the arena, allowing the buffer to be reused.
	//
	// All pointers obtained from the arena before Reset are invalidated.
	// Dereferencing them after Reset leads to undefined behavior — they
	// alias future allocations from the same buffer. The caller must
	// ensure that previously allocated objects are not used after Reset.
	//
	// For standalone arenas, Reset also releases all bulk pins (Pin).
	// For GPA-embedded arenas (created via GeneralPurposeAllocator.NewArena),
	// bulk pins live on the parent GPA and are NOT released by Reset —
	// use PinManaged + Unpin for those.
	Reset()

	destroy()
}

type arenaAllocator struct {
	buf       []byte
	cursor    int
	parent    Allocator
	refs      []any
	managed   map[int]any
	nextPinID int
}

// align is always a power of 2 (guaranteed by unsafe.Alignof), so the
// compiler optimizes cursor%align to a bitwise AND.
func (a *arenaAllocator) canAlloc(size, align int) bool {
	padding := (align - a.cursor%align) % align
	return a.cursor+padding+size <= len(a.buf)
}

func (a *arenaAllocator) alloc(size, align int) unsafe.Pointer {
	if !a.canAlloc(size, align) {
		return nil
	}
	padding := (align - a.cursor%align) % align
	start := a.cursor + padding
	ptr := unsafe.Pointer(&a.buf[start])
	a.cursor = start + size
	clear(a.buf[start : start+size])
	return ptr
}

func (a *arenaAllocator) free(ptr unsafe.Pointer) error {
	return nil
}

func (a *arenaAllocator) pin(value any) {
	if a.parent != nil {
		a.parent.pin(value)
		return
	}
	a.refs = append(a.refs, value)
}

func (a *arenaAllocator) pinManaged(value any) int {
	if a.parent != nil {
		return a.parent.pinManaged(value)
	}
	id := a.nextPinID
	a.nextPinID++
	if a.managed == nil {
		a.managed = make(map[int]any)
	}
	a.managed[id] = value
	return id
}

func (a *arenaAllocator) unpin(id int) {
	if a.parent != nil {
		a.parent.unpin(id)
		return
	}
	delete(a.managed, id)
}

func (a *arenaAllocator) destroy() {
	clear(a.buf)
	a.buf = nil
	a.cursor = 0
	a.refs = nil
	a.managed = nil
	a.parent = nil
}

func (a *arenaAllocator) Reset() {
	a.cursor = 0
	if a.parent == nil {
		a.refs = a.refs[:0]
		clear(a.managed)
	}
}

// NewArena returns a new arena allocator with a buffer of the given size.
func NewArena(size int64) Arena {
	return &arenaAllocator{
		buf: make([]byte, size),
	}
}

// NewArenaFrom returns a new arena allocator backed by a pre-allocated byte
// slice. The arena does not own the slice — the caller must ensure it
// outlives all pointers obtained from the arena.
func NewArenaFrom(buf []byte) Arena {
	return &arenaAllocator{
		buf: buf,
	}
}

// DestroyArena zeroes the arena's buffer, releases all pins, and sets the
// Arena variable to nil.
//
// Existing pointers into the arena still reference the underlying buffer
// (they now read as zeroes). The GC keeps the buffer alive until all such
// pointers go out of scope, which can cause memory leaks. Nil out
// arena-derived pointers when they are no longer needed.
func DestroyArena(arena *Arena) {
	aa := (*arena).(*arenaAllocator)
	parent := aa.parent
	aa.destroy()
	if parent != nil {
		parent.free(unsafe.Pointer(aa))
	}
	*arena = nil
}
