package mmm

import (
	"unsafe"
)

// Arena is an arena allocator.
type Arena interface {
	Allocator
	destroy()
}

type arenaAllocator struct {
	buf    []byte
	cursor int
	parent Allocator
}

func (a *arenaAllocator) canAlloc(size, align int) bool {
	remainder := a.cursor % align
	return a.cursor+size+remainder <= len(a.buf)
}

//TODO: check if this is faster:
// p := ptr
// modulo := p & (align-1)
// if modulo != 0 {
// 	p += align - modulo
// }
//
// maybe the compiler optimizes it to the same thing as:
// remainder := a.cursor % align

func (a *arenaAllocator) alloc(size, align int) unsafe.Pointer {
	if !a.canAlloc(size, align) {
		return nil
	}
	remainder := a.cursor % align
	ptr := unsafe.Pointer(&a.buf[a.cursor+remainder])
	a.cursor += size + remainder
	return ptr
}

func (a *arenaAllocator) free(ptr unsafe.Pointer) error {
	// no-op
	return nil
}

func (a *arenaAllocator) destroy() {
	a.buf = nil
	a.cursor = 0
	if a.parent != nil {
		a.parent.free(unsafe.Pointer(a))
		a.parent = nil
	}
}

// TODO: remove this
func (a *arenaAllocator) SetByte(index int, d byte) {
	a.buf[index] = d
}

// NewArena returns a new arena allocator.
func NewArena(size int64) Arena {
	return &arenaAllocator{
		buf: make([]byte, size),
	}
}

// NewArenaFrom returns a new arena allocator from a pre-allocated byte slice.
func NewArenaFrom(buf []byte) Arena {
	return &arenaAllocator{
		buf: buf,
	}
}

func DestroyArena(arena *Arena) {
	(*arena).destroy()
	*arena = nil
}
