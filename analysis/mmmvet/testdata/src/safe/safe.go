package safe

import "github.com/gabstv/mmm"

type ScalarOnly struct {
	A int
	B float64
	C [4]byte
}

func example() {
	var arena mmm.Allocator
	_ = mmm.Alloc[ScalarOnly](arena)
	_ = mmm.Alloc[int](arena)
	_ = mmm.Alloc[[8]byte](arena)
}
