package mmm

import (
	"testing"
	"unsafe"
)

// TestGrowingArena_BasicAllocation verifies a simple allocation stays within
// chunk 0 and returns a usable pointer.
func TestGrowingArena_BasicAllocation(t *testing.T) {
	arena := NewGrowingArena(256)
	defer DestroyGrowingArena(&arena)

	x := Alloc[int64](arena)
	*x = 42

	if arena.ChunkCount() != 1 {
		t.Fatalf("expected ChunkCount=1, got %d", arena.ChunkCount())
	}
	if *x != 42 {
		t.Fatalf("expected 42, got %d", *x)
	}
}

// TestGrowingArena_GrowsWhenChunkFull verifies that when chunk 0 is exhausted
// a second chunk is created and both pointers remain valid.
func TestGrowingArena_GrowsWhenChunkFull(t *testing.T) {
	const chunkSize = 64
	arena := NewGrowingArena(chunkSize)
	defer DestroyGrowingArena(&arena)

	// Fill chunk 0 (64 bytes, 8 int64s of 8 bytes each).
	ptrs := make([]*int64, 8)
	for i := range ptrs {
		ptrs[i] = Alloc[int64](arena)
		*ptrs[i] = int64(i)
	}

	if arena.ChunkCount() != 1 {
		t.Fatalf("expected ChunkCount=1 after filling chunk 0, got %d", arena.ChunkCount())
	}

	// One more allocation must trigger growth.
	extra := Alloc[int64](arena)
	*extra = 999

	if arena.ChunkCount() != 2 {
		t.Fatalf("expected ChunkCount=2 after growth, got %d", arena.ChunkCount())
	}

	// All original pointers must still be valid.
	for i, p := range ptrs {
		if *p != int64(i) {
			t.Fatalf("ptr[%d]: expected %d, got %d", i, i, *p)
		}
	}
	if *extra != 999 {
		t.Fatalf("extra: expected 999, got %d", *extra)
	}
}

// TestGrowingArena_OldPointersStableAcrossGrowth stores a value in chunk 0,
// forces several growth cycles, and verifies the original pointer is unchanged.
func TestGrowingArena_OldPointersStableAcrossGrowth(t *testing.T) {
	const chunkSize = 64
	arena := NewGrowingArena(chunkSize)
	defer DestroyGrowingArena(&arena)

	first := Alloc[int64](arena)
	*first = 0xDEADBEEF

	// Fill the rest of chunk 0 and push into several more chunks.
	for i := 0; i < 7; i++ {
		p := Alloc[int64](arena)
		*p = int64(i)
	}
	for i := 0; i < 3*8; i++ {
		p := Alloc[int64](arena)
		*p = int64(i)
	}

	if *first != 0xDEADBEEF {
		t.Fatalf("pointer into chunk 0 corrupted after growth: got %d", *first)
	}
}

// TestGrowingArena_OversizedAllocation verifies that a single allocation
// larger than chunkSize succeeds and the memory is usable.
func TestGrowingArena_OversizedAllocation(t *testing.T) {
	const chunkSize = 64
	arena := NewGrowingArena(chunkSize)
	defer DestroyGrowingArena(&arena)

	before := arena.ChunkCount()
	big := Alloc[[200]byte](arena)

	if arena.ChunkCount() <= before {
		t.Fatalf("expected ChunkCount to increase for oversized alloc, got %d", arena.ChunkCount())
	}

	// Write and read back all 200 bytes.
	for i := range big {
		big[i] = byte(i % 256)
	}
	for i := range big {
		if big[i] != byte(i%256) {
			t.Fatalf("big[%d]: expected %d, got %d", i, byte(i%256), big[i])
		}
	}
}

// TestGrowingArena_OversizedAllocationDoesNotStrandSubsequentSmallAlloc
// verifies that after an oversized chunk becomes active, a subsequent small
// allocation succeeds.
func TestGrowingArena_OversizedAllocationDoesNotStrandSubsequentSmallAlloc(t *testing.T) {
	const chunkSize = 64
	arena := NewGrowingArena(chunkSize)
	defer DestroyGrowingArena(&arena)

	_ = Alloc[[200]byte](arena)

	small := Alloc[int32](arena)
	*small = 7

	if *small != 7 {
		t.Fatalf("expected 7, got %d", *small)
	}
}

// TestGrowingArena_ResetRewindsWhenNeverGrew verifies that Reset on a
// single-chunk arena keeps the same underlying buffer.
func TestGrowingArena_ResetRewindsWhenNeverGrew(t *testing.T) {
	arena := NewGrowingArena(256)
	defer DestroyGrowingArena(&arena)

	ga := arena.(*growingArena)

	_ = Alloc[int64](arena)
	bufBefore := uintptr(unsafe.Pointer(&ga.chunks[0].buf[0]))

	arena.Reset()

	if arena.ChunkCount() != 1 {
		t.Fatalf("expected ChunkCount=1 after rewind reset, got %d", arena.ChunkCount())
	}
	bufAfter := uintptr(unsafe.Pointer(&ga.chunks[0].buf[0]))
	if bufBefore != bufAfter {
		t.Fatalf("buffer pointer changed on rewind reset: %x → %x", bufBefore, bufAfter)
	}
	if ga.chunks[0].cursor != 0 {
		t.Fatalf("cursor not reset to 0, got %d", ga.chunks[0].cursor)
	}
}

// TestGrowingArena_ResetCollapsesWhenGrew verifies that Reset on a multi-chunk
// arena collapses to a single chunk and updates HighWaterMark.
func TestGrowingArena_ResetCollapsesWhenGrew(t *testing.T) {
	const chunkSize = 64
	arena := NewGrowingArena(chunkSize)
	defer DestroyGrowingArena(&arena)

	// Grow to at least 3 chunks (24 × int64 = 192 bytes > 2×64).
	for i := 0; i < 24; i++ {
		p := Alloc[int64](arena)
		*p = int64(i)
	}
	if arena.ChunkCount() < 3 {
		t.Fatalf("expected at least 3 chunks before reset, got %d", arena.ChunkCount())
	}

	hwm := arena.HighWaterMark()
	arena.Reset()

	if arena.ChunkCount() != 1 {
		t.Fatalf("expected ChunkCount=1 after collapse reset, got %d", arena.ChunkCount())
	}
	// HighWaterMark after reset equals the new chunk size (the collapse target).
	if arena.HighWaterMark() > hwm {
		t.Fatalf("HighWaterMark increased unexpectedly after reset: %d > %d", arena.HighWaterMark(), hwm)
	}
}

// TestGrowingArena_ResetCollapseSizesToHighWaterMark verifies that the
// collapsed chunk is at least as large as the pre-reset high-water mark.
func TestGrowingArena_ResetCollapseSizesToHighWaterMark(t *testing.T) {
	const chunkSize = 64
	arena := NewGrowingArena(chunkSize)
	defer DestroyGrowingArena(&arena)

	// Allocate ~200+ bytes across chunks (chunkSize=64, so 3+ chunks).
	for i := 0; i < 25; i++ {
		p := Alloc[int64](arena)
		*p = 0
	}

	hwmBefore := arena.HighWaterMark()
	arena.Reset()

	ga := arena.(*growingArena)
	newLen := int64(len(ga.chunks[0].buf))

	if newLen < hwmBefore {
		t.Fatalf("collapsed chunk len %d < high-water mark %d", newLen, hwmBefore)
	}
}

// TestGrowingArena_WithMaxCollapseSize verifies that the collapse target is
// clamped to maxCollapseSize (provided it is >= chunkSize).
func TestGrowingArena_WithMaxCollapseSize(t *testing.T) {
	const chunkSize = 64
	const maxCollapse = 128
	arena := NewGrowingArena(chunkSize, WithMaxCollapseSize(maxCollapse))
	defer DestroyGrowingArena(&arena)

	// Drive high-water mark well above 128 bytes.
	for i := 0; i < 40; i++ {
		p := Alloc[int64](arena)
		*p = 0
	}
	if arena.HighWaterMark() <= maxCollapse {
		t.Fatalf("high-water mark %d did not exceed maxCollapse %d — test precondition failed",
			arena.HighWaterMark(), maxCollapse)
	}

	arena.Reset()

	ga := arena.(*growingArena)
	newLen := int64(len(ga.chunks[0].buf))
	if newLen != maxCollapse {
		t.Fatalf("expected collapsed chunk len=%d, got %d", maxCollapse, newLen)
	}
}

// TestGrowingArena_PinsClearedOnReset verifies that Reset clears bulk pins
// and managed pins without panicking, and the arena is usable afterwards.
func TestGrowingArena_PinsClearedOnReset(t *testing.T) {
	arena := NewGrowingArena(256)
	defer DestroyGrowingArena(&arena)

	for i := 0; i < 5; i++ {
		Pin(arena, "value")
	}
	_, h1 := PinManaged(arena, "managed1")
	_, h2 := PinManaged(arena, "managed2")
	_ = h1
	_ = h2

	ga := arena.(*growingArena)
	if len(ga.refs) != 5 {
		t.Fatalf("expected 5 bulk pins, got %d", len(ga.refs))
	}
	if len(ga.managed) != 2 {
		t.Fatalf("expected 2 managed pins, got %d", len(ga.managed))
	}

	arena.Reset()

	if len(ga.refs) != 0 {
		t.Fatalf("expected 0 refs after reset, got %d", len(ga.refs))
	}
	if len(ga.managed) != 0 {
		t.Fatalf("expected 0 managed pins after reset, got %d", len(ga.managed))
	}

	// Arena must still be usable.
	x := Alloc[int](arena)
	*x = 1
}

// TestGrowingArena_AllocReturnsAlignedPointers verifies that allocations
// respect their natural alignment requirements.
func TestGrowingArena_AllocReturnsAlignedPointers(t *testing.T) {
	arena := NewGrowingArena(256)
	defer DestroyGrowingArena(&arena)

	// Offset cursor by 1 byte.
	_ = Alloc[byte](arena)

	x := Alloc[int64](arena)
	if uintptr(unsafe.Pointer(x))%8 != 0 {
		t.Fatalf("int64 not 8-byte aligned: %p", x)
	}
	*x = 0xCAFEBABE
	if *x != 0xCAFEBABE {
		t.Fatal("unexpected value after aligned alloc")
	}
}

// TestGrowingArena_CanAllocAlwaysTrue verifies that canAlloc returns true even
// when the active chunk is completely full.
func TestGrowingArena_CanAllocAlwaysTrue(t *testing.T) {
	const chunkSize = 8
	arena := NewGrowingArena(chunkSize)
	defer DestroyGrowingArena(&arena)

	ga := arena.(*growingArena)

	// Fill chunk 0 entirely.
	_ = Alloc[int64](arena)

	// canAlloc must still report true (growth will happen on the next alloc).
	if !ga.canAlloc(8, 8) {
		t.Fatal("canAlloc returned false on a GrowingArena")
	}
}

// TestGrowingArena_DestroyClearsState verifies that DestroyGrowingArena sets
// the interface variable to nil.
func TestGrowingArena_DestroyClearsState(t *testing.T) {
	arena := NewGrowingArena(128)

	_ = Alloc[int64](arena)

	DestroyGrowingArena(&arena)

	if arena != nil {
		t.Fatal("expected arena to be nil after DestroyGrowingArena")
	}
}

// TestGrowingArena_HighWaterMarkTracksPeak verifies that HighWaterMark
// increases during growth and reflects the new baseline after Reset.
func TestGrowingArena_HighWaterMarkTracksPeak(t *testing.T) {
	const chunkSize = 64
	arena := NewGrowingArena(chunkSize)
	defer DestroyGrowingArena(&arena)

	hwm0 := arena.HighWaterMark()

	// Grow to trigger more chunks.
	for i := 0; i < 16; i++ {
		p := Alloc[int64](arena)
		*p = 0
	}

	hwm1 := arena.HighWaterMark()
	if hwm1 <= hwm0 {
		t.Fatalf("HighWaterMark did not grow: before=%d after=%d", hwm0, hwm1)
	}

	arena.Reset()

	hwm2 := arena.HighWaterMark()
	ga := arena.(*growingArena)
	newLen := int64(len(ga.chunks[0].buf))

	if hwm2 != newLen {
		t.Fatalf("after reset, HighWaterMark=%d should equal new chunk len=%d", hwm2, newLen)
	}
}

// TestGrowingArena_WorksWithExistingWrappers is a smoke test using the
// generic Alloc wrapper from allocator.go.
func TestGrowingArena_WorksWithExistingWrappers(t *testing.T) {
	type Point struct {
		X, Y float64
	}

	arena := NewGrowingArena(256)
	defer DestroyGrowingArena(&arena)

	p := Alloc[Point](arena)
	p.X = 1.5
	p.Y = 2.5

	if p.X != 1.5 || p.Y != 2.5 {
		t.Fatalf("unexpected values: %+v", *p)
	}

	arena.Reset()

	p2 := Alloc[Point](arena)
	if p2.X != 0 || p2.Y != 0 {
		t.Fatalf("expected zero after reset, got %+v", *p2)
	}

	p2.X = 3.0
	p2.Y = 4.0
	if p2.X != 3.0 || p2.Y != 4.0 {
		t.Fatalf("unexpected values after second alloc: %+v", *p2)
	}
}
