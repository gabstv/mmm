package assignwarn

import "github.com/gabstv/mmm"

type Entry struct {
	Name string
	ID   int
}

func example() {
	var arena mmm.Allocator
	s := mmm.Alloc[Entry](arena) // want `pointer-bearing fields`
	s.Name = "hello"              // want `without Pin`
	s.ID = 42                     // no warning: int is scalar
}
