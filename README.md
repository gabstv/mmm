# MMM - Manual Memory Management

Provides allocators for managing memory in Go.

WARNING: Experimental. Do not use in production!

## Allocators

### Arena

A linear (bump) allocator. Allocations advance a cursor through a fixed-size buffer. Individual `Free` calls are no-ops — memory is reclaimed in bulk via `Reset` or `DestroyArena`.

```go
arena := mmm.NewArena(256)
defer mmm.DestroyArena(&arena)

x := mmm.Alloc[int](arena)
*x = 123

y := mmm.Alloc[uint16](arena)
*y = 456

// Stack-backed arena (zero heap allocations)
var stackBuf [256]byte
arena2 := mmm.NewArenaFrom(stackBuf[:])
defer mmm.DestroyArena(&arena2)
```

### General Purpose Allocator (GPA)

A bucket-based allocator that supports individual `Free` calls. Freed memory within a bucket is tracked in a free list and reused for subsequent allocations. Adjacent free regions are coalesced automatically.

```go
gpa := mmm.NewGeneralPurposeAllocator(1024)

stuff := mmm.Alloc[[100]int](gpa)
defer mmm.Free(gpa, &stuff)

stuff[0] = -123
stuff[1] = 32

mmm.Scope(func() {
    score := mmm.Alloc[float64](gpa)
    defer mmm.Free(gpa, &score)
    *score = 321.48
})

fmt.Println("size:", gpa.Size())
fmt.Println("count:", gpa.Count())
```

Sub-arenas can be created from a GPA for temporary batch work:

```go
gpa := mmm.NewGeneralPurposeAllocator(1024)
arena := gpa.NewArena(4096)
defer mmm.DestroyArena(&arena)

// Fast bump allocations within the arena
for i := 0; i < 100; i++ {
    p := mmm.Alloc[float64](arena)
    *p = float64(i)
}
```

## GC Safety: Pointer-Bearing Types

Arena memory lives inside a `[]byte` buffer. **The Go garbage collector does not scan this buffer for pointers.** If you store a value with internal pointers (string, slice, map, chan, func, interface, or pointer fields) inside arena memory, the GC may collect the target, leaving a dangling reference.

### Safe types (no pinning needed)

Types with no internal pointers work without any extra steps:

- `int`, `int8`..`int64`, `uint`, `uint8`..`uint64`
- `float32`, `float64`, `bool`, `byte`, `rune`
- `[N]T` where T is any of the above
- Structs composed entirely of the above

### Unsafe types (pinning required)

Any type containing pointer-bearing fields **must** have those values pinned:

```go
type Entry struct {
    Name string   // string has an internal pointer — MUST pin
    ID   int      // safe, no pinning needed
}

s := mmm.Alloc[Entry](arena)
s.ID = 42

// Pin keeps the string visible to the GC
s.Name = mmm.Pin(arena, buildString())
```

### Pin vs PinManaged

| Function | Lifetime | Best for |
|---|---|---|
| `Pin[T](allocator, value) T` | Bulk — cleared on `Reset()` / `DestroyArena()` | Arena allocators |
| `PinManaged[T](allocator, value) (T, PinHandle)` | Individual — released via `handle.Unpin()` | GPA allocators |

**`Pin`** — simple inline use. All pins are released together when the arena resets or is destroyed.

```go
arena := mmm.NewArena(1024)
defer mmm.DestroyArena(&arena)

s := mmm.Alloc[Entry](arena)
s.Name = mmm.Pin(arena, "hello")

arena.Reset() // all pins released
```

**`PinManaged`** — for allocations with different lifetimes. Call `Unpin()` when the value is no longer stored in arena memory.

```go
gpa := mmm.NewGeneralPurposeAllocator(1024)

s := mmm.Alloc[Entry](gpa)
var h mmm.PinHandle
s.Name, h = mmm.PinManaged(gpa, "hello")

// ... use s ...

h.Unpin()
mmm.Free(gpa, &s)
```

> **Note:** `Free()` does **not** automatically release managed pins. Always call `Unpin()` on your `PinHandle` values to avoid keeping unnecessary GC references alive.

## Static Analysis: `mmmvet`

`mmmvet` is a `go vet`-compatible tool that detects unsafe usage patterns at compile time.

It checks for:
1. `Alloc[T]` / `TryAlloc[T]` where T has pointer-bearing fields (warns to use Pin/PinManaged)
2. Field assignments on arena-allocated variables where the field is pointer-bearing and the value is not wrapped in `Pin` or `PinManaged`

### Install

```bash
go install github.com/gabstv/mmm/cmd/mmmvet@latest
```

### Run

```bash
# Standalone
mmmvet ./...

# Via go vet
go vet -vettool=$(which mmmvet) ./...
```

### CI (GitHub Actions)

```yaml
- name: Install mmmvet
  run: go install github.com/gabstv/mmm/cmd/mmmvet@latest

- name: Run mmmvet
  run: mmmvet ./...
```

### Example output

```
entry.go:15:7: mmm.Alloc with pointer-bearing fields: Name
entry.go:16:2: assignment to arena-allocated field Name (string) without Pin/PinManaged
```

## Important Caveats

### Alloc panics on OOM

`Alloc` panics if the allocator is out of memory. Use `TryAlloc` for explicit error handling:

```go
ptr, err := mmm.TryAlloc[MyStruct](arena)
if err != nil {
    // handle out of memory
}
```

### Reset invalidates all pointers

After `arena.Reset()`, all previously returned pointers still reference the buffer but will be silently overwritten by future allocations. Do not use pointers obtained before a Reset.

### DestroyArena and lingering pointers

After `DestroyArena`, the buffer is zeroed but existing pointers keep the underlying memory alive (the GC sees them as interior pointers). This can cause memory leaks — set arena-derived pointers to nil when they are no longer needed.

### Not thread-safe

Allocators have no internal synchronization. Do not call `Alloc`, `Free`, `Reset`, or `Pin` concurrently from multiple goroutines on the same allocator.

### GPA free-list behavior

Freed memory within a GPA bucket is tracked and reused. However:
- Fragments smaller than 8 bytes from alignment padding are absorbed (not tracked)
- Memory is only fully reclaimed when **all** allocations in a bucket are freed

## Benchmarks

```
# Allocating 128KB per iteration:

goos: darwin
goarch: arm64
cpu: Apple M4
BenchmarkArenaAlloc128KB-10      1189942     1004 ns/op      14 B/op    0 allocs/op  # arena
BenchmarkNoArenaAlloc128KB-10     250442     4631 ns/op  131073 B/op    1 allocs/op  # heap
```
