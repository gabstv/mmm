package mslices_test

import (
	"testing"

	"github.com/gabstv/mmm"
	mslices "github.com/gabstv/mmm/slices"
)

func TestNewAligned(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.NewAligned[int32](arena, 10)
	if s == nil {
		t.Fatal("NewAligned returned nil")
	}
	if mslices.Len(s) != 0 {
		t.Fatalf("Len = %d, want 0", mslices.Len(s))
	}
	want := mslices.AlignedCap[int32](10)
	if mslices.Cap(s) != want {
		t.Fatalf("Cap = %d, want %d", mslices.Cap(s), want)
	}
}

func TestGrowInPlace(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[int64](arena, 4)
	for i := range 4 {
		mslices.Append(s, int64(i+1))
	}

	g := mslices.Grow[int64](arena, s, 16)
	if g == nil {
		t.Fatal("Grow returned nil")
	}
	if g != s {
		t.Fatal("expected in-place growth (same Slice)")
	}
	if mslices.Cap(g) < 16 {
		t.Fatalf("Cap = %d, want >= 16", mslices.Cap(g))
	}
	if mslices.Len(g) != 4 {
		t.Fatalf("Len = %d, want 4", mslices.Len(g))
	}
	for i := range 4 {
		if got := *mslices.Get[int64](g, i); got != int64(i+1) {
			t.Fatalf("elem %d = %d, want %d", i, got, i+1)
		}
	}
	// Newly grown capacity is usable.
	if !mslices.Append(g, int64(99)) {
		t.Fatal("append into grown cap failed")
	}
}

func TestGrowMovesAndPreserves(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[int32](arena, 4)
	for i := range 4 {
		mslices.Append(s, int32(i*10))
	}
	// Make s no longer the tail allocation so Grow must move it.
	_ = mslices.New[int32](arena, 2)

	g := mslices.Grow[int32](arena, s, 32)
	if g == nil {
		t.Fatal("Grow returned nil")
	}
	if g == s {
		t.Fatal("expected a moved Slice (s was not the tail)")
	}
	if mslices.Cap(g) < 32 {
		t.Fatalf("Cap = %d, want >= 32", mslices.Cap(g))
	}
	if mslices.Len(g) != 4 {
		t.Fatalf("Len = %d, want 4", mslices.Len(g))
	}
	for i := range 4 {
		if got := *mslices.Get[int32](g, i); got != int32(i*10) {
			t.Fatalf("elem %d = %d, want %d", i, got, i*10)
		}
	}
}

func TestGrowNoShrink(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[int32](arena, 10)
	g := mslices.Grow[int32](arena, s, 5)
	if g != s {
		t.Fatal("Grow with smaller cap should return the same Slice unchanged")
	}
	if mslices.Cap(g) != 10 {
		t.Fatalf("Cap = %d, want 10", mslices.Cap(g))
	}
}

func TestGrowOOM(t *testing.T) {
	// Tight arena: fit the first slice and a blocker, but not a moved grow.
	arena := mmm.NewArena(96)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[int64](arena, 2)
	if s == nil {
		t.Fatal("New returned nil")
	}
	mslices.Append(s, 7)
	// Block in-place growth so Grow must move (and fail for lack of room).
	_ = mslices.New[int64](arena, 1)

	g := mslices.Grow[int64](arena, s, 100)
	if g != nil {
		t.Fatalf("expected nil on OOM, got %v", g)
	}
	// Original slice is still intact.
	if mslices.Len(s) != 1 || *mslices.Get[int64](s, 0) != 7 {
		t.Fatal("original slice corrupted after failed Grow")
	}
}
