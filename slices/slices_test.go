package mslices_test

import (
	"testing"
	"unsafe"

	"github.com/gabstv/mmm"
	mslices "github.com/gabstv/mmm/slices"
)

func TestNew(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[int32](arena, 10)
	if s == nil {
		t.Fatal("New returned nil")
	}
	if mslices.Len(s) != 0 {
		t.Fatalf("Len = %d, want 0", mslices.Len(s))
	}
	if mslices.Cap(s) != 10 {
		t.Fatalf("Cap = %d, want 10", mslices.Cap(s))
	}
}

func TestAppendAndGet(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[float32](arena, 5)
	for i := range 5 {
		if !mslices.Append[float32](s, float32(i)*1.5) {
			t.Fatalf("Append %d failed", i)
		}
	}
	if mslices.Len(s) != 5 {
		t.Fatalf("Len = %d, want 5", mslices.Len(s))
	}
	for i := range 5 {
		want := float32(i) * 1.5
		got := *mslices.Get[float32](s, i)
		if got != want {
			t.Errorf("Get(%d) = %f, want %f", i, got, want)
		}
	}
}

func TestAppendOverflow(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[int64](arena, 2)
	mslices.Append[int64](s, 1)
	mslices.Append[int64](s, 2)
	if mslices.Append[int64](s, 3) {
		t.Fatal("Append should return false at capacity")
	}
	if mslices.Len(s) != 2 {
		t.Fatalf("Len = %d, want 2", mslices.Len(s))
	}
}

func TestSet(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[int32](arena, 3)
	mslices.Append[int32](s, 10)
	mslices.Append[int32](s, 20)
	mslices.Append[int32](s, 30)

	mslices.Set[int32](s, 1, 99)
	if got := *mslices.Get[int32](s, 1); got != 99 {
		t.Fatalf("Get(1) = %d, want 99", got)
	}
}

func TestFrom(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	data := []float64{1.1, 2.2, 3.3, 4.4}
	s := mslices.From[float64](arena, data)
	if mslices.Len(s) != 4 {
		t.Fatalf("Len = %d, want 4", mslices.Len(s))
	}
	for i, want := range data {
		got := *mslices.Get[float64](s, i)
		if got != want {
			t.Errorf("Get(%d) = %f, want %f", i, got, want)
		}
	}
}

func TestGoSlice(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[int32](arena, []int32{10, 20, 30})
	gs := mslices.GoSlice[int32](s)
	if len(gs) != 3 {
		t.Fatalf("len = %d, want 3", len(gs))
	}
	if gs[0] != 10 || gs[1] != 20 || gs[2] != 30 {
		t.Fatalf("GoSlice = %v", gs)
	}
}

func TestAppendSlice(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[uint16](arena, 10)
	mslices.Append[uint16](s, 1)
	if !mslices.AppendSlice[uint16](s, []uint16{2, 3, 4, 5}) {
		t.Fatal("AppendSlice failed")
	}
	if mslices.Len(s) != 5 {
		t.Fatalf("Len = %d, want 5", mslices.Len(s))
	}
	gs := mslices.GoSlice[uint16](s)
	for i, want := range []uint16{1, 2, 3, 4, 5} {
		if gs[i] != want {
			t.Errorf("element %d = %d, want %d", i, gs[i], want)
		}
	}
}

func TestAppendSliceOverflow(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[int32](arena, 3)
	mslices.Append[int32](s, 1)
	if mslices.AppendSlice[int32](s, []int32{2, 3, 4}) {
		t.Fatal("AppendSlice should return false when exceeding capacity")
	}
	if mslices.Len(s) != 1 {
		t.Fatalf("Len = %d, want 1 (unchanged after failed append)", mslices.Len(s))
	}
}

func TestClear(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[int32](arena, []int32{1, 2, 3})
	mslices.Clear(s)
	if mslices.Len(s) != 0 {
		t.Fatalf("Len = %d, want 0", mslices.Len(s))
	}
	if mslices.Cap(s) != 3 {
		t.Fatalf("Cap = %d, want 3 (unchanged)", mslices.Cap(s))
	}
}

func TestTruncate(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[int32](arena, []int32{1, 2, 3, 4, 5})
	mslices.Truncate(s, 3)
	if mslices.Len(s) != 3 {
		t.Fatalf("Len = %d, want 3", mslices.Len(s))
	}
	if *mslices.Get[int32](s, 2) != 3 {
		t.Fatalf("Get(2) = %d, want 3", *mslices.Get[int32](s, 2))
	}
}

func TestTruncatePanic(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[int32](arena, []int32{1, 2})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Truncate(3) should panic")
		}
	}()
	mslices.Truncate(s, 3)
}

func TestGetPanic(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[int32](arena, []int32{1})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Get(1) should panic on slice of len 1")
		}
	}()
	mslices.Get[int32](s, 1)
}

func TestSetPanic(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[int32](arena, []int32{1})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Set(1) should panic on slice of len 1")
		}
	}()
	mslices.Set[int32](s, 1, 99)
}

func TestPtr(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[float32](arena, []float32{1.0, 2.0, 3.0})
	p := mslices.Ptr[float32](s)
	if p == nil {
		t.Fatal("Ptr returned nil")
	}
	if *p != 1.0 {
		t.Fatalf("*Ptr = %f, want 1.0", *p)
	}
	// Walk contiguous memory via pointer arithmetic
	p2 := (*float32)(unsafe.Add(unsafe.Pointer(p), 4))
	if *p2 != 2.0 {
		t.Fatalf("Ptr[1] = %f, want 2.0", *p2)
	}
	p3 := (*float32)(unsafe.Add(unsafe.Pointer(p), 8))
	if *p3 != 3.0 {
		t.Fatalf("Ptr[2] = %f, want 3.0", *p3)
	}
}

func TestPtrEmpty(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[int32](arena, 5)
	if mslices.Ptr[int32](s) != nil {
		t.Fatal("Ptr on empty slice should be nil")
	}
}

type Vec3 struct {
	X, Y, Z float32
}

func TestStructElements(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.New[Vec3](arena, 10)
	mslices.Append[Vec3](s, Vec3{1, 2, 3})
	mslices.Append[Vec3](s, Vec3{4, 5, 6})

	v := mslices.Get[Vec3](s, 1)
	if v.X != 4 || v.Y != 5 || v.Z != 6 {
		t.Fatalf("Get(1) = %+v", *v)
	}

	// Mutate through pointer
	v.X = 99
	if mslices.Get[Vec3](s, 1).X != 99 {
		t.Fatal("mutation through Get pointer didn't stick")
	}
}

func TestGPA(t *testing.T) {
	gpa := mmm.NewGeneralPurposeAllocator(4096)

	s1 := mslices.From[int32](gpa, []int32{1, 2, 3})
	s2 := mslices.From[int32](gpa, []int32{4, 5, 6})

	if *mslices.Get[int32](s1, 0) != 1 {
		t.Fatal("s1[0] wrong")
	}
	if *mslices.Get[int32](s2, 0) != 4 {
		t.Fatal("s2[0] wrong")
	}

	if err := mslices.Free(gpa, s1); err != nil {
		t.Fatal(err)
	}
	if *mslices.Get[int32](s2, 0) != 4 {
		t.Fatal("s2[0] wrong after freeing s1")
	}
	if err := mslices.Free(gpa, s2); err != nil {
		t.Fatal(err)
	}
}

func TestAlignedCap(t *testing.T) {
	tests := []struct {
		name string
		f    func(int) int
		es   int
		aa   int
	}{
		{"byte", mslices.AlignedCap[byte], 1, 4},
		{"uint16", mslices.AlignedCap[uint16], 2, 4},
		{"int32", mslices.AlignedCap[int32], 4, 4},
		{"float64", mslices.AlignedCap[float64], 8, 8},
		{"[3]byte", mslices.AlignedCap[[3]byte], 3, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for cap := range 33 {
				got := tt.f(cap)
				if got < cap {
					t.Errorf("AlignedCap(%d) = %d, less than input", cap, got)
				}
				total := 16 + got*tt.es // header(16) + elements (dataOffset==16 for all these types)
				if total%tt.aa != 0 {
					t.Errorf("AlignedCap(%d) = %d: total %d not aligned to %d", cap, got, total, tt.aa)
				}
			}
		})
	}
}

func TestAlignedCapEliminatesPadding(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	cap1 := mslices.AlignedCap[byte](10)
	s1 := mslices.New[byte](arena, cap1)
	s2 := mslices.New[byte](arena, cap1)
	s3 := mslices.New[byte](arena, cap1)

	addr1 := uintptr(unsafe.Pointer(s1))
	addr2 := uintptr(unsafe.Pointer(s2))
	addr3 := uintptr(unsafe.Pointer(s3))

	stride1 := addr2 - addr1
	stride2 := addr3 - addr2
	if stride1 != stride2 {
		t.Fatalf("inconsistent stride: %d vs %d", stride1, stride2)
	}
}

func TestMultipleSlicesContiguous(t *testing.T) {
	arena := mmm.NewArena(1 << 16)
	defer mmm.DestroyArena(&arena)

	slices := make([]mslices.Slice, 100)
	for i := range slices {
		slices[i] = mslices.From[int32](arena, []int32{int32(i), int32(i * 10)})
		if slices[i] == nil {
			t.Fatalf("allocation %d returned nil", i)
		}
	}
	for i, s := range slices {
		if *mslices.Get[int32](s, 0) != int32(i) {
			t.Fatalf("slice %d element 0 = %d", i, *mslices.Get[int32](s, 0))
		}
		if *mslices.Get[int32](s, 1) != int32(i*10) {
			t.Fatalf("slice %d element 1 = %d", i, *mslices.Get[int32](s, 1))
		}
	}
}

func TestGetMutatesInPlace(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mslices.From[int64](arena, []int64{100, 200, 300})
	ptr := mslices.Get[int64](s, 1)
	*ptr = 999
	if *mslices.Get[int64](s, 1) != 999 {
		t.Fatal("in-place mutation through Get didn't work")
	}
}
