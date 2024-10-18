# MMM - Manual Memory Management

Provides allocators for managing memory in Go.

WARNING: Experimental. Do not use in production!

## Example -> GPA

```go
package main

import (
    "fmt"

    "github.com/gabstv/mmm"
)

func main() {
    gpa := mmm.NewGeneralPurposeAllocator(1024)

    defer func(a mmm.GeneralPurposeAllocator){
        fmt.Println("final size:", a.Size())
        fmt.Println("final count:", a.Count())
    }(gpa)

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
}
```

## Example -> Arena

```go
package main

import (
	"fmt"

	"github.com/gabstv/mmm"
)

func main() {
	arena1 := mmm.NewArena(256)

	var stackBuffer [256]byte
	arena2 := mmm.NewArenaFrom(stackBuffer[:])

	defer mmm.DestroyArena(&arena1)
	defer mmm.DestroyArena(&arena2)

	x := mmm.Alloc[int](arena1)
	*x = 123

	y := mmm.Alloc[uint16](arena1)
	*y = 456

	z := mmm.Alloc[bool](arena2)
	*z = true

	fmt.Printf("x: %d, y: %d, z: %t\n", *x, *y, *z)
}
```

Benchmarks:

```
# Allocating 128KB of memory:

goos: linux
goarch: amd64
pkg: github.com/gabstv/mmm
cpu: AMD Ryzen 5 4500 6-Core Processor              
BenchmarkArenaAlloc128KB-12      	29451156	        38.42 ns/op	       0 B/op	       0 allocs/op # arena
BenchmarkNoArenaAlloc128KB-12    	   70413	     28048 ns/op	  131072 B/op	       1 allocs/op # heap
```