package mmm

import "unsafe"

type Allocator interface {
	alloc(size, align int) unsafe.Pointer
	canAlloc(size, align int) bool
	free(ptr unsafe.Pointer) error
}

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

func Alloc[T any](allocator Allocator) *T {
	var zt T
	sz := unsafe.Sizeof(zt)
	az := unsafe.Alignof(zt)

	pp := allocator.alloc(int(sz), int(az))

	return (*T)(pp)
}

func Free[T any](allocator Allocator, ptr **T) error {
	if err := allocator.free(unsafe.Pointer(*ptr)); err != nil {
		return err
	}

	*ptr = nil

	return nil
}
