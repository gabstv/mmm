package mmm

import (
	"runtime"
	"testing"
	"unsafe"
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

		for i := range 1024 {
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

// TestGPAFreeListReuse verifies that freeing A and then allocating C (same
// size) reuses A's slot — the bucket count stays at 1 and Size() is unchanged.
func TestGPAFreeListReuse(t *testing.T) {
	// Use a bucket large enough to hold all three int64 allocations.
	gpa := NewGeneralPurposeAllocator(256)

	a := Alloc[int64](gpa)
	*a = 111
	b := Alloc[int64](gpa)
	*b = 222

	sizeAfterAB := gpa.Size()
	if gpa.Count() != 2 {
		t.Fatalf("expected count 2, got %d", gpa.Count())
	}

	// Free A; B is still live so the bucket must stay.
	ptrA := unsafe.Pointer(a)
	Free(gpa, &a)
	if gpa.Count() != 1 {
		t.Fatalf("after free A: expected count 1, got %d", gpa.Count())
	}
	// Size should be unchanged — bucket still exists because B is live.
	if gpa.Size() != sizeAfterAB {
		t.Fatalf("after free A: size changed unexpectedly: %d vs %d", gpa.Size(), sizeAfterAB)
	}

	// Allocate C with the same size as A.
	c := Alloc[int64](gpa)
	*c = 333

	// C should occupy A's old slot, so Size() must not have grown.
	if gpa.Size() != sizeAfterAB {
		t.Fatalf("after alloc C: size grew — free list reuse did not happen: %d vs %d", gpa.Size(), sizeAfterAB)
	}
	if gpa.Count() != 2 {
		t.Fatalf("after alloc C: expected count 2, got %d", gpa.Count())
	}

	// C must reside at the same address as the old A.
	if unsafe.Pointer(c) != ptrA {
		t.Fatalf("C was not placed at A's old address (free list reuse failed)")
	}

	Free(gpa, &b)
	Free(gpa, &c)

	if gpa.Count() != 0 {
		t.Fatalf("expected count 0 after all frees, got %d", gpa.Count())
	}
	if gpa.Size() != 0 {
		t.Fatalf("expected size 0 after all frees, got %d", gpa.Size())
	}
}

// TestGPAFreeListCoalescing verifies that freeing two adjacent allocations
// coalesces them into a single free region that can serve a larger allocation.
func TestGPAFreeListCoalescing(t *testing.T) {
	// 128-byte bucket; each int64 is 8 bytes.
	gpa := NewGeneralPurposeAllocator(128)

	a := Alloc[int64](gpa)
	b := Alloc[int64](gpa)
	c := Alloc[int64](gpa)

	_ = c // keep c live so the bucket stays

	// Free A then B — they are adjacent so they must coalesce into a 16-byte
	// free region that can serve a [2]int64 (16 bytes) allocation.
	Free(gpa, &a)
	Free(gpa, &b)

	// Verify there is exactly one free region and it is 16 bytes.
	gi := gpa.(*generalPurposeAllocator)
	if len(gi.buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(gi.buckets))
	}
	bucket := &gi.buckets[0]
	if len(bucket.freeRegions) != 1 {
		t.Fatalf("expected 1 coalesced free region, got %d", len(bucket.freeRegions))
	}
	if bucket.freeRegions[0].size != 16 {
		t.Fatalf("coalesced region size: expected 16, got %d", bucket.freeRegions[0].size)
	}

	// A [2]int64 fits in the coalesced region.
	d := Alloc[[2]int64](gpa)
	(*d)[0] = 1
	(*d)[1] = 2

	if gpa.Count() != 2 { // c and d
		t.Fatalf("expected count 2 after alloc d, got %d", gpa.Count())
	}

	Free(gpa, &c)
	Free(gpa, &d)

	if gpa.Count() != 0 {
		t.Fatalf("expected count 0 after all frees, got %d", gpa.Count())
	}
}

// TestGPATailReclaim verifies that freeing the last bump-allocated object
// moves the cursor back so the space can be reused immediately.
func TestGPATailReclaim(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(256)

	a := Alloc[int64](gpa)
	*a = 1

	b := Alloc[int64](gpa)
	*b = 2

	gi := gpa.(*generalPurposeAllocator)
	cursorAfterAB := gi.buckets[0].cursor

	// Save B's raw address before Free nulls the pointer.
	ptrB := unsafe.Pointer(b)

	// Free B (the tail allocation) — cursor should retreat.
	Free(gpa, &b)

	cursorAfterFreeB := gi.buckets[0].cursor
	if cursorAfterFreeB >= cursorAfterAB {
		t.Fatalf("cursor did not retreat after freeing tail: before=%d after=%d",
			cursorAfterAB, cursorAfterFreeB)
	}
	// No free regions should remain — the space was returned to the bump zone.
	if len(gi.buckets[0].freeRegions) != 0 {
		t.Fatalf("expected 0 free regions after tail reclaim, got %d",
			len(gi.buckets[0].freeRegions))
	}

	// Re-allocate to confirm reuse — must land at B's old address.
	b2 := Alloc[int64](gpa)
	if unsafe.Pointer(b2) != ptrB {
		t.Fatalf("b2 address %p != old b address %p — tail reclaim reuse failed", b2, ptrB)
	}
	// Cursor must be back where it was after A and B were both allocated.
	if gi.buckets[0].cursor != cursorAfterAB {
		t.Fatalf("cursor after re-alloc: expected %d, got %d", cursorAfterAB, gi.buckets[0].cursor)
	}

	Free(gpa, &a)
	Free(gpa, &b2)

	if gpa.Count() != 0 {
		t.Fatalf("expected count 0 after all frees, got %d", gpa.Count())
	}
	if gpa.Size() != 0 {
		t.Fatalf("expected size 0 after all frees, got %d", gpa.Size())
	}
}

// TestGPAInterleavedAllocFree verifies Count() and Size() stay correct
// throughout a mixed alloc/free pattern.
func TestGPAInterleavedAllocFree(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(512)

	ptrs := make([]*int64, 0, 20)
	for i := range 10 {
		p := Alloc[int64](gpa)
		*p = int64(i)
		ptrs = append(ptrs, p)
	}
	if gpa.Count() != 10 {
		t.Fatalf("expected count 10, got %d", gpa.Count())
	}

	// Free even-indexed allocations
	for i := 0; i < 10; i += 2 {
		Free(gpa, &ptrs[i])
	}
	if gpa.Count() != 5 {
		t.Fatalf("expected count 5 after partial free, got %d", gpa.Count())
	}

	// Alloc 5 more — should reuse freed slots
	sizeBefore := gpa.Size()
	for i := range 5 {
		p := Alloc[int64](gpa)
		*p = int64(100 + i)
		ptrs = append(ptrs, p)
	}
	if gpa.Count() != 10 {
		t.Fatalf("expected count 10 after re-alloc, got %d", gpa.Count())
	}
	if gpa.Size() != sizeBefore {
		t.Fatalf("size grew after re-alloc — free list reuse did not happen: %d vs %d",
			gpa.Size(), sizeBefore)
	}

	// Free everything
	for i := range ptrs {
		if ptrs[i] != nil {
			Free(gpa, &ptrs[i])
		}
	}
	if gpa.Count() != 0 {
		t.Fatalf("expected count 0 after all frees, got %d", gpa.Count())
	}
	if gpa.Size() != 0 {
		t.Fatalf("expected size 0 after all frees, got %d", gpa.Size())
	}
}

// TestGPANewArenaWithFreeList verifies that NewArena still works correctly
// alongside the free-list mechanism — create an arena, allocate from it,
// destroy it, and verify GPA state is clean.
func TestGPANewArenaWithFreeList(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(256)

	// Alloc a normal value first
	x := Alloc[int64](gpa)
	*x = 42

	// Create a sub-arena from the GPA
	arena := gpa.NewArena(128)

	p := Alloc[int64](arena)
	*p = 99

	q := Alloc[int32](arena)
	*q = 7

	// GPA count should be 2: x + the arena struct allocation
	if gpa.Count() != 2 {
		t.Fatalf("expected count 2 (x + arena), got %d", gpa.Count())
	}

	// Destroy the arena — this frees its backing allocation from the GPA
	DestroyArena(&arena)

	if gpa.Count() != 1 {
		t.Fatalf("after destroy arena: expected count 1, got %d", gpa.Count())
	}

	// Free x
	Free(gpa, &x)

	if gpa.Count() != 0 {
		t.Fatalf("expected count 0 after all frees, got %d", gpa.Count())
	}
	if gpa.Size() != 0 {
		t.Fatalf("expected size 0 after all frees, got %d", gpa.Size())
	}
}

func TestCustomGPAMallocFree(t *testing.T) {
	var allocated []unsafe.Pointer

	mallocfn := func(size int) unsafe.Pointer {
		buf := make([]byte, size)
		ptr := unsafe.Pointer(&buf[0])
		allocated = append(allocated, ptr)
		runtime.KeepAlive(buf)
		return ptr
	}

	var freed []unsafe.Pointer
	freefn := func(ptr unsafe.Pointer, size int) {
		freed = append(freed, ptr)
	}

	gpa := NewCustomGeneralPurposeAllocator(128, mallocfn, freefn)

	x := Alloc[int](gpa)
	*x = 42

	if len(allocated) != 1 {
		t.Fatalf("expected 1 malloc call, got %d", len(allocated))
	}

	Free(gpa, &x)

	if len(freed) != 1 {
		t.Fatalf("expected 1 free call after bucket emptied, got %d", len(freed))
	}
	if freed[0] != allocated[0] {
		t.Fatal("freed pointer does not match allocated pointer")
	}
}

func TestCustomGPADestroy(t *testing.T) {
	var allocCount, freeCount int

	mallocfn := func(size int) unsafe.Pointer {
		allocCount++
		buf := make([]byte, size)
		ptr := unsafe.Pointer(&buf[0])
		runtime.KeepAlive(buf)
		return ptr
	}
	freefn := func(ptr unsafe.Pointer, size int) {
		freeCount++
	}

	gpa := NewCustomGeneralPurposeAllocator(64, mallocfn, freefn)

	_ = Alloc[int](gpa)
	_ = Alloc[[128]byte](gpa)

	if allocCount != 2 {
		t.Fatalf("expected 2 malloc calls, got %d", allocCount)
	}

	gpa.Destroy()

	if freeCount != 2 {
		t.Fatalf("expected 2 free calls from Destroy, got %d", freeCount)
	}
}

func TestGPANewGrowingArena(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(4096)

	ga := gpa.NewGrowingArena(256)

	// Allocate values in the growing arena — backed by GPA memory.
	p := Alloc[int64](ga)
	*p = 123
	q := Alloc[int32](ga)
	*q = 456

	if *p != 123 || *q != 456 {
		t.Fatalf("unexpected values: p=%d q=%d", *p, *q)
	}

	// GPA should have one allocation (the 256-byte initial chunk).
	if gpa.Count() != 1 {
		t.Fatalf("expected GPA count 1 (initial chunk), got %d", gpa.Count())
	}

	DestroyGrowingArena(&ga)

	if gpa.Count() != 0 {
		t.Fatalf("expected GPA count 0 after destroy, got %d", gpa.Count())
	}
}

func TestGPANewGrowingArenaGrowth(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(4096)

	ga := gpa.NewGrowingArena(64)

	if ga.ChunkCount() != 1 {
		t.Fatalf("expected 1 chunk, got %d", ga.ChunkCount())
	}

	// Allocate more than one chunk's worth to trigger growth.
	for range 10 {
		p := Alloc[[16]byte](ga)
		p[0] = 0xFF
	}

	if ga.ChunkCount() < 2 {
		t.Fatalf("expected growth, got %d chunks", ga.ChunkCount())
	}

	// All chunk allocations come from the GPA.
	if gpa.Count() < 2 {
		t.Fatalf("expected GPA count >= 2 (multiple chunks), got %d", gpa.Count())
	}

	// Reset collapses into a single chunk — old chunks freed, new one allocated.
	ga.Reset()

	if ga.ChunkCount() != 1 {
		t.Fatalf("expected 1 chunk after reset, got %d", ga.ChunkCount())
	}
	if gpa.Count() != 1 {
		t.Fatalf("expected GPA count 1 after reset collapse, got %d", gpa.Count())
	}

	DestroyGrowingArena(&ga)

	if gpa.Count() != 0 {
		t.Fatalf("expected GPA count 0 after destroy, got %d", gpa.Count())
	}
}

func TestGPANewGrowingArenaPinDelegation(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(4096)

	ga := gpa.NewGrowingArena(256)

	s := "hello from growing arena"
	Pin(ga, s)

	// Pin went to the GPA, not the growing arena's local refs.
	// Destroy the growing arena — the pin survives on the GPA.
	DestroyGrowingArena(&ga)

	// Pin still alive on GPA; destroy GPA to clean up.
	gpa.Destroy()
}

func TestGoBackedGPADestroy(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(128)

	_ = Alloc[int](gpa)
	_ = Alloc[int](gpa)

	gpa.Destroy()

	if gpa.Count() != 0 {
		t.Fatalf("expected count 0 after Destroy, got %d", gpa.Count())
	}
	if gpa.Size() != 0 {
		t.Fatalf("expected size 0 after Destroy, got %d", gpa.Size())
	}
}
