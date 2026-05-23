package mmm

import (
	"runtime"
	"testing"
	"time"
	"unsafe"
)

type TestMangle struct {
	A int
	B bool
	C string
}

func TestArena(t *testing.T) {
	arena := NewArena(64)
	defer DestroyArena(&arena)

	x := Alloc[int](arena)
	*x = 123

	y := Alloc[uint16](arena)
	*y = 456

	ok := Alloc[bool](arena)
	*ok = true

	tm, err := TryAlloc[TestMangle](arena)

	if err != nil {
		t.Fatal(err)
	}

	tm.A = 123
	tm.B = true
	tm.C = "hello"

	ok2, _ := TryAlloc[bool](arena)
	*ok2 = true
	ok3, _ := TryAlloc[bool](arena)
	*ok3 = true
	ok4, _ := TryAlloc[bool](arena)
	*ok4 = true

	t.Logf("x: %d, y: %d, ok: %d, tm: %p, ok2: %d, ok3: %d, ok4: %d", x, y, ok, tm, ok2, ok3, ok4)
	t.Logf("tm: %p %s", &tm.C, tm.C)
	time.Sleep(time.Second)
	runtime.GC()
	time.Sleep(time.Second)
	t.Logf("tm: %p %s", &tm.C, tm.C)

	arena.(*arenaAllocator).buf[0] = 22

	t.Logf("x: %d, y: %d", *x, *y)

	if *x != 22 {
		t.Fail()
	}

	slc := Alloc[[5]byte](arena)
	slc[0] = 1
	slc[1] = 2
	slc[2] = 3
	slc[3] = 4
	slc[4] = 5

	if arena.(*arenaAllocator).cursor != 56 {
		t.Fatalf("expected cursor=56, got %d", arena.(*arenaAllocator).cursor)
	}

	// this should fail because the arena max size is 64 bytes:
	bigslice, err := TryAlloc[[32]byte](arena)

	if err == nil {
		t.Fail()
	}

	if bigslice != nil {
		t.Fail()
	}

	// this should also fail:
	bigslice2, err2 := TryAlloc[[32]int](arena)

	if err2 == nil {
		t.Fail()
	}

	if bigslice2 != nil {
		t.Fail()
	}
}

func TestAllocPanicsOnOOM(t *testing.T) {
	arena := NewArena(8)
	defer DestroyArena(&arena)

	_ = Alloc[int](arena) // fills the arena

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from Alloc on full arena")
		}
	}()
	Alloc[int](arena) // should panic
}

func TestArenaAlignment(t *testing.T) {
	arena := NewArena(256)
	defer DestroyArena(&arena)

	// Offset cursor by 1 byte to force alignment padding
	_ = Alloc[byte](arena) // cursor → 1

	x := Alloc[int64](arena) // needs 8-byte alignment
	if uintptr(unsafe.Pointer(x))%8 != 0 {
		t.Fatalf("int64 not 8-byte aligned: %p", x)
	}

	*x = 42
	if *x != 42 {
		t.Fatal("unexpected value")
	}

	// One more: offset by 3, then alloc a uint32 (align 4)
	_ = Alloc[byte](arena)
	_ = Alloc[byte](arena)
	_ = Alloc[byte](arena)
	y := Alloc[uint32](arena)
	if uintptr(unsafe.Pointer(y))%4 != 0 {
		t.Fatalf("uint32 not 4-byte aligned: %p", y)
	}

	*y = 0xDEADBEEF
	if *y != 0xDEADBEEF {
		t.Fatal("unexpected value")
	}
}

func TestArenaZeroed(t *testing.T) {
	arena := NewArena(64)
	defer DestroyArena(&arena)

	x := Alloc[int](arena)
	if *x != 0 {
		t.Fatalf("expected zero, got %d", *x)
	}

	*x = 999
	arena.Reset()

	x2 := Alloc[int](arena)
	if *x2 != 0 {
		t.Fatalf("after reset, expected zero, got %d", *x2)
	}
}

func TestArenaPin(t *testing.T) {
	arena := NewArena(256)
	defer DestroyArena(&arena)

	type HasString struct {
		Value string
	}

	s := Alloc[HasString](arena)
	// Heap-allocated string (not a literal — literals are in rodata, never GC'd)
	s.Value = Pin(arena, string([]byte{'h', 'e', 'l', 'l', 'o'}))

	runtime.GC()
	runtime.GC()

	if s.Value != "hello" {
		t.Fatalf("expected 'hello', got %q", s.Value)
	}
}

func TestArenaPinManaged(t *testing.T) {
	arena := NewArena(256)
	defer DestroyArena(&arena)

	type HasString struct {
		Value string
	}

	s := Alloc[HasString](arena)
	var h PinHandle
	s.Value, h = PinManaged(arena, string([]byte{'w', 'o', 'r', 'l', 'd'}))

	runtime.GC()
	runtime.GC()

	if s.Value != "world" {
		t.Fatalf("expected 'world', got %q", s.Value)
	}

	// Unpin and verify handle is consumed
	h.Unpin()
	h.Unpin() // double-unpin is safe
}

func TestArenaPinClearedOnReset(t *testing.T) {
	arena := NewArena(256)
	defer DestroyArena(&arena)

	a := arena.(*arenaAllocator)

	Pin(arena, "keep1")
	Pin(arena, "keep2")
	if len(a.refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(a.refs))
	}

	arena.Reset()
	if len(a.refs) != 0 {
		t.Fatalf("expected 0 refs after reset, got %d", len(a.refs))
	}
}

func BenchmarkArenaAlloc128KB(b *testing.B) {
	var heapbuf [1024 * 1024 * 16]byte // 16MB

	arena := NewArenaFrom(heapbuf[:])
	var sink byte
	b.ResetTimer()
	for b.Loop() {
	realloc:
		x, err := TryAlloc[[65536 * 2]byte](arena) // 128KB

		if err != nil {
			// out of memory, let's reset the arena for this benchmark
			arena.Reset()
			goto realloc
		}

		sink = x[0]
	}
	_ = sink
}

func BenchmarkNoArenaAlloc128KB(b *testing.B) {
	var sink byte
	for b.Loop() {
		x := make([]byte, 65536*2) // 128KB - large enough to escape to the heap
		// 128KB is defined as the max stack var size in https://github.com/golang/go/blob/master/src/cmd/compile/internal/gc/main.go#L132

		sink = x[0]
	}
	_ = sink
}
