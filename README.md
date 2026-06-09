# MMM - Manual Memory Management

Provides allocators for managing memory in Go.

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

### Custom GPA with CGo-backed memory

For CGo-heavy applications, you can create a GPA whose bucket memory is allocated by C, avoiding copies when passing data to C functions:

```go
/*
#include <stdlib.h>
#include <string.h>
*/
import "C"

import "unsafe"

func main() {
    gpa := mmm.NewCustomGeneralPurposeAllocator(
        4096,
        func(size int) unsafe.Pointer {
            return C.malloc(C.size_t(size))
        },
        func(ptr unsafe.Pointer, size int) {
            C.free(ptr)
        },
    )
    defer gpa.Destroy()

    // Allocations live in C memory — pass directly to C functions
    data := mmm.Alloc[[512]byte](gpa)
    C.memset(unsafe.Pointer(data), 0xFF, 512)

    mmm.Free(gpa, &data)
}
```

Sub-arenas created from a custom GPA inherit the C allocator:

```go
arena := gpa.NewArena(8192)
defer mmm.DestroyArena(&arena)

// These bump allocations also live in C memory
p := mmm.Alloc[MyVertex](arena)
```

> **Note:** You must call `Destroy()` when the GPA is no longer needed. Go's GC will not free C-allocated memory. For Go-backed GPAs, `Destroy()` is optional but recommended — it nils internal state so post-destroy use panics immediately.

## Arena-Allocated Data Types

### Strings (`mstrings`)

Arena-backed strings with null termination for C interop. Zero GC pressure.

```go
import mstrings "github.com/gabstv/mmm/strings"

arena := mmm.NewArena(4096)
defer mmm.DestroyArena(&arena)

s := mstrings.From(arena, "hello world")
fmt.Println(mstrings.GoString(s)) // "hello world"
```

Strings implement `encoding.TextMarshaler` and `encoding.TextUnmarshaler`, so they serialize as regular JSON strings — indistinguishable from Go's `string`:

```go
data, _ := json.Marshal(s)       // "hello world"
json.Unmarshal(data, s)           // writes directly into arena
```

### Slices (`mslices`)

Generic arena-allocated slices with fixed capacity.

```go
import mslices "github.com/gabstv/mmm/slices"

s := mslices.From[float32](arena, []float32{1.0, 2.0, 3.0})
mslices.Append[float32](s, 4.0)
fmt.Println(mslices.GoSlice[float32](s)) // [1 2 3 4]
```

### Ring Buffers (`mring`)

Fixed-capacity ring buffers for bounded history (input combos, position trails, event logs).

```go
import mring "github.com/gabstv/mmm/ring"

r := mring.New[int](arena, 4)
for i := range 6 {
    mring.Push[int](r, i) // overwrites oldest when full
}
// r contains [2, 3, 4, 5]
```

### Timestamps (`mtime`)

Arena-allocated timestamps with microsecond precision and timezone preservation. The `Timestamp` value type is 16 bytes with zero GC pointers — safe to store directly in `mslices.Slice[Timestamp]` or `mring.Ring[Timestamp]` without pinning.

```go
import mtime "github.com/gabstv/mmm/time"

arena := mmm.NewArena(4096)
defer mmm.DestroyArena(&arena)

// Arena-allocated (Time = *Timestamp)
mt := mtime.From(arena, time.Now())
fmt.Println(mt.GoTime()) // reconstructs full time.Time with timezone

// Value type (no arena needed)
ts := mtime.NewTimestamp(time.Now())
fmt.Println(ts.Unix())
```

Timestamps implement `json.Marshaler` and `json.Unmarshaler`. The default format is RFC 3339 (matching `time.Time`), with a global switch for Unix micro/milliseconds:

```go
data, _ := json.Marshal(mt)              // "2025-06-15T10:30:00.123456Z"

mtime.SetJSONFormat(mtime.FormatUnixMicro)
data, _ = json.Marshal(mt)               // 1750070400123456

// UnmarshalJSON always accepts RFC 3339 strings; for numeric values it
// uses the current format setting (or auto-detects for post-2001 dates)
json.Unmarshal(data, mt)
```

Timezone identity is preserved through a package-level registry. Round-tripping through `GoTime()` returns a `time.Time` with the original IANA zone (including DST rules), not just a fixed UTC offset.

### JSON for Slices and Ring Buffers

Slices and ring buffers are generic types, so they can't implement `json.Marshaler` directly (the header erases the type parameter). Instead, the packages provide generic helper functions:

```go
data, err := mslices.MarshalJSON[float32](s)
err = mslices.UnmarshalJSON[float32](s, data)

data, err = mring.MarshalJSON[int](r)
err = mring.UnmarshalJSON[int](r, data)
```

To get automatic `json.Marshal`/`json.Unmarshal` support, define a named type that delegates to the helpers:

```go
type Positions struct{ mslices.Slice }

func (p Positions) MarshalJSON() ([]byte, error)   { return mslices.MarshalJSON[Vec2](p.Slice) }
func (p Positions) UnmarshalJSON(b []byte) error    { return mslices.UnmarshalJSON[Vec2](p.Slice, b) }
```

Now `Positions` works transparently with `json.Marshal`/`json.Unmarshal` and serializes as a standard JSON array.

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
