package mslices_test

import (
	"slices"
	"testing"

	"github.com/gabstv/mmm"
	mslices "github.com/gabstv/mmm/slices"
)

func TestAll(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[int](arena, []int{10, 20, 30})

	var indices []int
	var values []int
	for i, v := range mslices.All[int](s) {
		indices = append(indices, i)
		values = append(values, v)
	}
	if !slices.Equal(indices, []int{0, 1, 2}) {
		t.Fatalf("indices = %v", indices)
	}
	if !slices.Equal(values, []int{10, 20, 30}) {
		t.Fatalf("values = %v", values)
	}
}

func TestValues(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[int](arena, []int{1, 2, 3})

	var values []int
	for v := range mslices.Values[int](s) {
		values = append(values, v)
	}
	if !slices.Equal(values, []int{1, 2, 3}) {
		t.Fatalf("values = %v", values)
	}
}

func TestBackward(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[int](arena, []int{10, 20, 30})

	var indices []int
	var values []int
	for i, v := range mslices.Backward[int](s) {
		indices = append(indices, i)
		values = append(values, v)
	}
	if !slices.Equal(indices, []int{2, 1, 0}) {
		t.Fatalf("indices = %v", indices)
	}
	if !slices.Equal(values, []int{30, 20, 10}) {
		t.Fatalf("values = %v", values)
	}
}

func TestIteratorBreak(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[int](arena, []int{1, 2, 3, 4, 5})

	count := 0
	for range mslices.Values[int](s) {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Fatalf("early break: count = %d, want 3", count)
	}
}

func TestEmptyIterator(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[int](arena, 10) // len 0

	count := 0
	for range mslices.Values[int](s) {
		count++
	}
	if count != 0 {
		t.Fatalf("empty iterator yielded %d elements", count)
	}
}
