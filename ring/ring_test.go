package mring_test

import (
	"slices"
	"testing"

	"github.com/gabstv/mmm"
	mring "github.com/gabstv/mmm/ring"
)

func TestNewEmpty(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int32](arena, 8)
	if r == nil {
		t.Fatal("New returned nil")
	}
	if mring.Len(r) != 0 {
		t.Fatalf("Len = %d, want 0", mring.Len(r))
	}
	if mring.Cap(r) != 8 {
		t.Fatalf("Cap = %d, want 8", mring.Cap(r))
	}
	if !mring.Empty(r) {
		t.Fatal("should be empty")
	}
	if mring.Full(r) {
		t.Fatal("should not be full")
	}
}

func TestPushPop(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)
	mring.Push[int](r, 10)
	mring.Push[int](r, 20)
	mring.Push[int](r, 30)

	if mring.Len(r) != 3 {
		t.Fatalf("Len = %d, want 3", mring.Len(r))
	}

	v, ok := mring.Pop[int](r)
	if !ok || v != 10 {
		t.Fatalf("Pop = %d, %v; want 10, true", v, ok)
	}
	v, ok = mring.Pop[int](r)
	if !ok || v != 20 {
		t.Fatalf("Pop = %d, %v; want 20, true", v, ok)
	}
	v, ok = mring.Pop[int](r)
	if !ok || v != 30 {
		t.Fatalf("Pop = %d, %v; want 30, true", v, ok)
	}
	_, ok = mring.Pop[int](r)
	if ok {
		t.Fatal("Pop on empty should return false")
	}
}

func TestCapRoundsUp(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	tests := []struct{ input, want int }{
		{1, 1}, {2, 2}, {3, 4}, {4, 4},
		{5, 8}, {7, 8}, {8, 8}, {9, 16},
		{15, 16}, {16, 16}, {17, 32},
	}
	for _, tt := range tests {
		r := mring.New[byte](arena, tt.input)
		if mring.Cap(r) != tt.want {
			t.Errorf("New(_, %d): Cap = %d, want %d", tt.input, mring.Cap(r), tt.want)
		}
	}
}

func TestPushOverwrite(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)
	mring.Push[int](r, 1)
	mring.Push[int](r, 2)
	mring.Push[int](r, 3)
	mring.Push[int](r, 4)
	if !mring.Full(r) {
		t.Fatal("should be full")
	}

	// Overwrite oldest (1)
	mring.Push[int](r, 5)
	if mring.Len(r) != 4 {
		t.Fatalf("Len = %d, want 4", mring.Len(r))
	}

	// Should now contain [2, 3, 4, 5]
	v, _ := mring.Pop[int](r)
	if v != 2 {
		t.Fatalf("got %d, want 2", v)
	}
	v, _ = mring.Pop[int](r)
	if v != 3 {
		t.Fatalf("got %d, want 3", v)
	}
	v, _ = mring.Pop[int](r)
	if v != 4 {
		t.Fatalf("got %d, want 4", v)
	}
	v, _ = mring.Pop[int](r)
	if v != 5 {
		t.Fatalf("got %d, want 5", v)
	}
}

func TestTryPush(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 2)
	if !mring.TryPush[int](r, 1) {
		t.Fatal("TryPush should succeed")
	}
	if !mring.TryPush[int](r, 2) {
		t.Fatal("TryPush should succeed")
	}
	if mring.TryPush[int](r, 3) {
		t.Fatal("TryPush on full should return false")
	}
	if mring.Len(r) != 2 {
		t.Fatalf("Len = %d, want 2", mring.Len(r))
	}
}

func TestPeek(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)

	_, ok := mring.Peek[int](r)
	if ok {
		t.Fatal("Peek on empty should return false")
	}

	mring.Push[int](r, 10)
	mring.Push[int](r, 20)
	v, ok := mring.Peek[int](r)
	if !ok || v != 10 {
		t.Fatalf("Peek = %d, %v; want 10, true", v, ok)
	}
	if mring.Len(r) != 2 {
		t.Fatal("Peek should not change length")
	}
}

func TestPeekBack(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)

	_, ok := mring.PeekBack[int](r)
	if ok {
		t.Fatal("PeekBack on empty should return false")
	}

	mring.Push[int](r, 10)
	mring.Push[int](r, 20)
	mring.Push[int](r, 30)

	v, ok := mring.PeekBack[int](r)
	if !ok || v != 30 {
		t.Fatalf("PeekBack = %d, %v; want 30, true", v, ok)
	}
}

func TestGetSet(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)
	mring.Push[int](r, 10)
	mring.Push[int](r, 20)
	mring.Push[int](r, 30)

	// Index 0 = oldest (10), index 2 = newest (30)
	if *mring.Get[int](r, 0) != 10 {
		t.Fatalf("Get(0) = %d, want 10", *mring.Get[int](r, 0))
	}
	if *mring.Get[int](r, 2) != 30 {
		t.Fatalf("Get(2) = %d, want 30", *mring.Get[int](r, 2))
	}

	mring.Set[int](r, 1, 99)
	if *mring.Get[int](r, 1) != 99 {
		t.Fatalf("Get(1) after Set = %d, want 99", *mring.Get[int](r, 1))
	}
}

func TestGetAfterWrap(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)
	mring.Push[int](r, 1)
	mring.Push[int](r, 2)
	mring.Push[int](r, 3)
	mring.Push[int](r, 4)
	mring.Push[int](r, 5) // overwrites 1 → [2,3,4,5]
	mring.Push[int](r, 6) // overwrites 2 → [3,4,5,6]
	mring.Push[int](r, 7) // overwrites 3 → [4,5,6,7]

	if *mring.Get[int](r, 0) != 4 {
		t.Fatalf("Get(0) = %d, want 4", *mring.Get[int](r, 0))
	}
	if *mring.Get[int](r, 1) != 5 {
		t.Fatalf("Get(1) = %d, want 5", *mring.Get[int](r, 1))
	}
	if *mring.Get[int](r, 2) != 6 {
		t.Fatalf("Get(2) = %d, want 6", *mring.Get[int](r, 2))
	}
	if *mring.Get[int](r, 3) != 7 {
		t.Fatalf("Get(3) = %d, want 7", *mring.Get[int](r, 3))
	}
}

func TestGetPanic(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)
	mring.Push[int](r, 1)
	defer func() {
		if recover() == nil {
			t.Fatal("Get(1) should panic on ring of len 1")
		}
	}()
	mring.Get[int](r, 1)
}

func TestClear(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)
	mring.Push[int](r, 1)
	mring.Push[int](r, 2)
	mring.Clear(r)

	if mring.Len(r) != 0 {
		t.Fatalf("Len = %d, want 0", mring.Len(r))
	}
	if !mring.Empty(r) {
		t.Fatal("should be empty after Clear")
	}
}

type InputEvent struct {
	Frame  uint32
	Button uint8
	_      [3]byte // padding
}

func TestStructElements(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[InputEvent](arena, 16)
	for i := range 20 {
		mring.Push[InputEvent](r, InputEvent{Frame: uint32(i), Button: uint8(i % 4)})
	}
	// Should have last 16 entries (frames 4..19)
	if mring.Len(r) != 16 {
		t.Fatalf("Len = %d, want 16", mring.Len(r))
	}
	oldest := mring.Get[InputEvent](r, 0)
	if oldest.Frame != 4 {
		t.Fatalf("oldest frame = %d, want 4", oldest.Frame)
	}
	newest := mring.Get[InputEvent](r, 15)
	if newest.Frame != 19 {
		t.Fatalf("newest frame = %d, want 19", newest.Frame)
	}
}

func TestAllIterator(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)
	mring.Push[int](r, 10)
	mring.Push[int](r, 20)
	mring.Push[int](r, 30)

	var indices []int
	var values []int
	for i, v := range mring.All[int](r) {
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

func TestAllIteratorAfterWrap(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)
	for i := range 6 {
		mring.Push[int](r, (i+1)*10)
	}
	// Pushed 10,20,30,40,50,60 into cap-4 ring → [30,40,50,60]

	var values []int
	for _, v := range mring.All[int](r) {
		values = append(values, v)
	}
	if !slices.Equal(values, []int{30, 40, 50, 60}) {
		t.Fatalf("values = %v, want [30, 40, 50, 60]", values)
	}
}

func TestValuesIterator(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)
	mring.Push[int](r, 1)
	mring.Push[int](r, 2)
	mring.Push[int](r, 3)

	var values []int
	for v := range mring.Values[int](r) {
		values = append(values, v)
	}
	if !slices.Equal(values, []int{1, 2, 3}) {
		t.Fatalf("values = %v", values)
	}
}

func TestBackwardIterator(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)
	mring.Push[int](r, 1)
	mring.Push[int](r, 2)
	mring.Push[int](r, 3)

	var indices []int
	var values []int
	for i, v := range mring.Backward[int](r) {
		indices = append(indices, i)
		values = append(values, v)
	}
	if !slices.Equal(indices, []int{2, 1, 0}) {
		t.Fatalf("indices = %v", indices)
	}
	if !slices.Equal(values, []int{3, 2, 1}) {
		t.Fatalf("values = %v", values)
	}
}

func TestIteratorBreak(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 8)
	for i := range 8 {
		mring.Push[int](r, i)
	}

	count := 0
	for _, v := range mring.All[int](r) {
		_ = v
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

	r := mring.New[int](arena, 4)
	count := 0
	for range mring.Values[int](r) {
		count++
	}
	if count != 0 {
		t.Fatalf("empty iterator yielded %d elements", count)
	}
}

func TestGPA(t *testing.T) {
	gpa := mmm.NewGeneralPurposeAllocator(4096)

	r := mring.New[int](gpa, 4)
	mring.Push[int](r, 1)
	mring.Push[int](r, 2)

	v, _ := mring.Pop[int](r)
	if v != 1 {
		t.Fatalf("got %d, want 1", v)
	}

	if err := mring.Free(gpa, r); err != nil {
		t.Fatal(err)
	}
}

func TestPopPushCycle(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	r := mring.New[int](arena, 4)
	// Fill, drain, refill — exercises head wrapping via Pop
	for i := range 4 {
		mring.Push[int](r, i)
	}
	for range 4 {
		mring.Pop[int](r)
	}
	for i := range 4 {
		mring.Push[int](r, i+100)
	}

	var values []int
	for _, v := range mring.All[int](r) {
		values = append(values, v)
	}
	if !slices.Equal(values, []int{100, 101, 102, 103}) {
		t.Fatalf("values = %v, want [100, 101, 102, 103]", values)
	}
}
