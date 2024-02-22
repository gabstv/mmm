package mmm

type Error string

func (e Error) Error() string {
	return string(e)
}

const (
	ErrOutOfMemory = Error("out of memory")
	ErrNotFound    = Error("not found")
)
