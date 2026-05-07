package allocwarn

import "github.com/gabstv/mmm"

type HasString struct {
	Value string
}

type HasSlice struct {
	Items []int
}

type HasPointer struct {
	Ref *int
}

func example() {
	var arena mmm.Allocator
	_ = mmm.Alloc[HasString](arena)  // want `pointer-bearing fields`
	_ = mmm.Alloc[HasSlice](arena)   // want `pointer-bearing fields`
	_ = mmm.Alloc[HasPointer](arena) // want `pointer-bearing fields`
}
