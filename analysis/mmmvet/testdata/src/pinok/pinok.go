package pinok

import "github.com/gabstv/mmm"

type Entry struct {
	Name string
}

func example() {
	var arena mmm.Allocator
	s := mmm.Alloc[Entry](arena)             // want `pointer-bearing fields`
	s.Name = mmm.Pin(arena, "hello")          // no warning
	s.Name, _ = mmm.PinManaged(arena, "hi")  // no warning
}
