package mmm

import (
	"runtime"
	"testing"
)

func TestGeneralPurposeAllocator(t *testing.T) {
	allocator := NewGeneralPurposeAllocator(128)

	x := Alloc[int](allocator)
	*x = 123

	y := Alloc[uint16](allocator)
	*y = 456

	largeItem := Alloc[[1024]int](allocator)

	if allocator.Count() != 3 {
		t.Fail()
	}

	Free(allocator, &x)

	if allocator.Count() != 2 {
		t.Fail()
	}

	Free(allocator, &largeItem)

	if allocator.Count() != 1 {
		t.Fail()
	}

	if allocator.Size() != 128 {
		t.Fail()
	}

	Free(allocator, &y)

	if allocator.Count() != 0 {
		t.Fail()
	}

	if allocator.Size() != 0 {
		t.Fail()
	}
}

func TestArenaInsideGPA(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(128)

	arena1 := gpa.NewArena(64)
	arena2 := gpa.NewArena(1024)

	x := Alloc[int](arena1)
	*x = 123

	y := Alloc[uint16](arena1)
	*y = 456

	z := Alloc[bool](arena2)
	*z = true

	f := Alloc[[12]int](arena2)

	Free(arena2, &f)
	Free(arena2, &z)
	DestroyArena(&arena2)

	if gpa.Count() != 1 {
		t.Fail()
	}

	DestroyArena(&arena1)

	if gpa.Count() != 0 {
		t.Fail()
	}

	if gpa.Size() != 0 {
		t.Fail()
	}
}

func TestGPAAllocs(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(1024)

	Scope(func() {
		temp1 := Alloc[float64](gpa)
		defer Free(gpa, &temp1)
		*temp1 = 123.456

		temp2 := Alloc[float64](gpa)
		defer Free(gpa, &temp2)
		*temp2 = 789.012

		temp3 := Alloc[float64](gpa)
		defer Free(gpa, &temp3)
		*temp3 = *temp1 + *temp2

		if *temp3 != 912.468 {
			t.Fail()
		}

		tempArena := gpa.NewArena(1024 * 8)
		defer DestroyArena(&tempArena)

		for i := 0; i < 1024; i++ {
			temp4 := Alloc[float64](tempArena)
			*temp4 = float64(i)
		}
	})

	if gpa.Count() != 0 {
		t.Fatal("memory leaked")
	}

	if gpa.Size() != 0 {
		t.Fail()
	}
}

func TestGPAPinManaged(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(256)

	type Entry struct {
		Name string
	}

	e := Alloc[Entry](gpa)
	var h PinHandle
	e.Name, h = PinManaged(gpa, string([]byte{'t', 'e', 's', 't'}))

	runtime.GC()
	runtime.GC()

	if e.Name != "test" {
		t.Fatalf("expected 'test', got %q", e.Name)
	}

	// Unpin before free
	h.Unpin()
	Free(gpa, &e)

	if gpa.Count() != 0 {
		t.Fatal("expected 0 allocations after free")
	}
}

func TestGPAPinManagedMultiple(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(256)

	type Entry struct {
		First string
		Last  string
	}

	e := Alloc[Entry](gpa)
	var h1, h2 PinHandle
	e.First, h1 = PinManaged(gpa, string([]byte{'J', 'o', 'h', 'n'}))
	e.Last, h2 = PinManaged(gpa, string([]byte{'D', 'o', 'e'}))

	runtime.GC()

	if e.First != "John" || e.Last != "Doe" {
		t.Fatalf("expected John Doe, got %q %q", e.First, e.Last)
	}

	// Free the allocation and release pins
	h1.Unpin()
	h2.Unpin()
	Free(gpa, &e)

	g := gpa.(*generalPurposeAllocator)
	if len(g.managed) != 0 {
		t.Fatalf("expected 0 managed pins, got %d", len(g.managed))
	}
}
