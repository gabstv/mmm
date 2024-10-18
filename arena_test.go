package mmm

import (
	"runtime"
	"testing"
	"time"
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

	if arena.(*arenaAllocator).cursor != 54 {
		t.Fail()
	}

	// this should fail because the arena max size is 64 bytes:
	bigslice, err := TryAlloc[[32]byte](arena)

	if err == nil {
		t.Fail()
	}

	if bigslice != nil {
		t.Fail()
	}

	// this should fail silently:
	bigslice2 := Alloc[[32]int](arena)

	if bigslice2 != nil {
		t.Fail()
	}
}

func BenchmarkArenaAlloc128KB(b *testing.B) {
	var heapbuf [1024 * 1024 * 16]byte // 16MB

	arena := NewArenaFrom(heapbuf[:])

	for i := 0; i < b.N; i++ {
	realloc:
		x := Alloc[[65536 * 2]byte](arena) // 128KB

		if x == nil {
			// out of memory, let's reset the arena for this benchmark
			arena.Reset()
			goto realloc
		}

		x[0] = byte(i) // truncated to 0-255
	}
}

func BenchmarkNoArenaAlloc128KB(b *testing.B) {
	for i := 0; i < b.N; i++ {
		x := make([]byte, 65536*2) // 128KB - large enough to escape to the heap
		// 128KB is defined as the max stack var size in https://github.com/golang/go/blob/master/src/cmd/compile/internal/gc/main.go#L132

		x[0] = byte(i) // truncated to 0-255
	}
}
