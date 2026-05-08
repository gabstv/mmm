package arenaptrsafe

import (
	"github.com/gabstv/mmm"
	mslices "github.com/gabstv/mmm/slices"
	mstrings "github.com/gabstv/mmm/strings"
)

// PureArena has only arena-safe pointer fields — no warnings expected.
type PureArena struct {
	Name mstrings.String
	Pos  mslices.Slice
	HP   int
}

// Mixed has both arena pointers and a GC string.
type Mixed struct {
	Label mstrings.String
	Desc  string
}

func pureArena() {
	var arena mmm.Allocator
	e := mmm.Alloc[PureArena](arena) // no warning: all pointer fields are arena-safe
	e.Name = nil                      // no warning: arena-safe type
	e.Pos = nil                       // no warning: arena-safe type
	e.HP = 42
}

func mixed() {
	var arena mmm.Allocator
	m := mmm.Alloc[Mixed](arena) // want `pointer-bearing fields`
	m.Label = nil                 // no warning: arena-safe type
	m.Desc = "oops"               // want `without Pin`
	m.Desc = mmm.Pin(arena, "ok") // no warning: pinned
}
