package mmm

type Allocator interface{}
type Arena interface{}
type PinHandle struct{}

func (*PinHandle) Unpin() {}

func NewArena(size int64) Arena                            { return nil }
func Alloc[T any](a Allocator) *T                         { return nil }
func TryAlloc[T any](a Allocator) (*T, error)             { return nil, nil }
func Pin[T any](a Allocator, v T) T                       { return v }
func PinManaged[T any](a Allocator, v T) (T, PinHandle)   { return v, PinHandle{} }
func Free[T any](a Allocator, ptr **T) error              { return nil }
