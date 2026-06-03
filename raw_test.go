package mmm

import (
	"testing"
	"unsafe"
)

func TestRawReallocInPlaceArena(t *testing.T) {
	arena := NewArena(256)
	defer DestroyArena(&arena)

	ptr := RawAlloc(arena, 16, 8)
	if ptr == nil {
		t.Fatal("initial alloc failed")
	}
	buf := unsafe.Slice((*byte)(ptr), 16)
	for i := range buf {
		buf[i] = byte(i + 1)
	}

	cursorBefore := arena.(*arenaAllocator).cursor

	np := RawRealloc(arena, ptr, 16, 64, 8)
	if np == nil {
		t.Fatal("realloc failed")
	}
	if np != ptr {
		t.Fatalf("expected in-place growth (same ptr), got new ptr")
	}
	// Old data preserved.
	nbuf := unsafe.Slice((*byte)(np), 64)
	for i := 0; i < 16; i++ {
		if nbuf[i] != byte(i+1) {
			t.Fatalf("data not preserved at %d: got %d", i, nbuf[i])
		}
	}
	// Cursor advanced by exactly the growth.
	if got := arena.(*arenaAllocator).cursor; got != cursorBefore-16+64 {
		t.Fatalf("unexpected cursor %d", got)
	}
}

func TestRawReallocFallbackArena(t *testing.T) {
	arena := NewArena(256)
	defer DestroyArena(&arena)

	ptr := RawAlloc(arena, 16, 8)
	if ptr == nil {
		t.Fatal("initial alloc failed")
	}
	buf := unsafe.Slice((*byte)(ptr), 16)
	for i := range buf {
		buf[i] = byte(i + 100)
	}

	// Make a second allocation so ptr is no longer the tail.
	_ = RawAlloc(arena, 8, 8)

	np := RawRealloc(arena, ptr, 16, 32, 8)
	if np == nil {
		t.Fatal("realloc failed")
	}
	if np == ptr {
		t.Fatal("expected a moved allocation (not tail), got same ptr")
	}
	nbuf := unsafe.Slice((*byte)(np), 32)
	for i := 0; i < 16; i++ {
		if nbuf[i] != byte(i+100) {
			t.Fatalf("data not preserved at %d: got %d", i, nbuf[i])
		}
	}
}

func TestRawReallocFallbackGPAReusesOldBlock(t *testing.T) {
	gpa := NewGeneralPurposeAllocator(1024)

	a := RawAlloc(gpa, 32, 8) // tail
	if a == nil {
		t.Fatal("alloc a failed")
	}
	abuf := unsafe.Slice((*byte)(a), 32)
	for i := range abuf {
		abuf[i] = byte(i + 1)
	}
	b := RawAlloc(gpa, 32, 8) // now b is the tail, a is not
	if b == nil {
		t.Fatal("alloc b failed")
	}

	// Growing a cannot be in place (not the tail). Fallback should alloc-new,
	// copy, and free the old block — reclaiming a's region in the free list.
	na := RawRealloc(gpa, a, 32, 64, 8)
	if na == nil {
		t.Fatal("realloc failed")
	}
	if na == a {
		t.Fatal("expected a moved allocation")
	}
	nbuf := unsafe.Slice((*byte)(na), 64)
	for i := 0; i < 32; i++ {
		if nbuf[i] != byte(i+1) {
			t.Fatalf("data not preserved at %d: got %d", i, nbuf[i])
		}
	}

	// The old 32-byte region of a should now be reusable. Allocate something
	// that fits in 32 bytes and confirm it lands inside the original bucket
	// (i.e. the free region was reclaimed, not orphaned).
	reuse := RawAlloc(gpa, 16, 8)
	if reuse == nil {
		t.Fatal("reuse alloc failed")
	}
	if uintptr(reuse) != uintptr(a) {
		t.Fatalf("expected reuse of freed block at %p, got %p", a, reuse)
	}
}

func TestRawReallocInPlaceGrowingArena(t *testing.T) {
	arena := NewGrowingArena(256)
	defer DestroyGrowingArena(&arena)

	ptr := RawAlloc(arena, 16, 8)
	if ptr == nil {
		t.Fatal("initial alloc failed")
	}
	buf := unsafe.Slice((*byte)(ptr), 16)
	for i := range buf {
		buf[i] = byte(i + 1)
	}

	// ptr is the tail of the active chunk — should grow in place.
	np := RawRealloc(arena, ptr, 16, 64, 8)
	if np == nil {
		t.Fatal("realloc failed")
	}
	if np != ptr {
		t.Fatalf("expected in-place growth (same ptr), got new ptr")
	}
	nbuf := unsafe.Slice((*byte)(np), 64)
	for i := 0; i < 16; i++ {
		if nbuf[i] != byte(i+1) {
			t.Fatalf("data not preserved at %d: got %d", i, nbuf[i])
		}
	}
}

func TestRawReallocFallbackGrowingArena(t *testing.T) {
	arena := NewGrowingArena(256)
	defer DestroyGrowingArena(&arena)

	ptr := RawAlloc(arena, 16, 8)
	if ptr == nil {
		t.Fatal("initial alloc failed")
	}
	buf := unsafe.Slice((*byte)(ptr), 16)
	for i := range buf {
		buf[i] = byte(i + 50)
	}

	// Make a second allocation so ptr is no longer the tail.
	_ = RawAlloc(arena, 8, 8)

	np := RawRealloc(arena, ptr, 16, 32, 8)
	if np == nil {
		t.Fatal("realloc failed")
	}
	if np == ptr {
		t.Fatal("expected a moved allocation (not tail), got same ptr")
	}
	nbuf := unsafe.Slice((*byte)(np), 32)
	for i := 0; i < 16; i++ {
		if nbuf[i] != byte(i+50) {
			t.Fatalf("data not preserved at %d: got %d", i, nbuf[i])
		}
	}
}

func TestRawReallocInPlaceAligned(t *testing.T) {
	arena := NewArena(256)
	defer DestroyArena(&arena)

	// Consume 1 byte so the next align-16 allocation has padding.
	_ = RawAlloc(arena, 1, 1)

	ptr := RawAlloc(arena, 16, 16)
	if ptr == nil {
		t.Fatal("initial alloc failed")
	}
	if uintptr(ptr)%16 != 0 {
		t.Fatalf("ptr not 16-byte aligned: %p", ptr)
	}
	buf := unsafe.Slice((*byte)(ptr), 16)
	for i := range buf {
		buf[i] = byte(i + 1)
	}

	// ptr is the tail — should grow in place even with alignment padding.
	np := RawRealloc(arena, ptr, 16, 48, 16)
	if np == nil {
		t.Fatal("realloc failed")
	}
	if np != ptr {
		t.Fatalf("expected in-place growth, got different ptr")
	}
	nbuf := unsafe.Slice((*byte)(np), 48)
	for i := 0; i < 16; i++ {
		if nbuf[i] != byte(i+1) {
			t.Fatalf("data not preserved at %d: got %d", i, nbuf[i])
		}
	}
}

func TestRawReallocNilPtr(t *testing.T) {
	arena := NewArena(64)
	defer DestroyArena(&arena)
	p := RawRealloc(arena, nil, 0, 16, 8)
	if p == nil {
		t.Fatal("expected alloc on nil ptr")
	}
}

func TestRawReallocOOM(t *testing.T) {
	arena := NewArena(32)
	defer DestroyArena(&arena)

	ptr := RawAlloc(arena, 16, 8)
	_ = RawAlloc(arena, 8, 8) // block in-place growth (ptr no longer tail)

	// Need 64 bytes total via fallback, but arena only has 32.
	np := RawRealloc(arena, ptr, 16, 64, 8)
	if np != nil {
		t.Fatalf("expected nil on OOM, got %p", np)
	}
}
