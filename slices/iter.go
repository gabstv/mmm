package mslices

import (
	"iter"
	"unsafe"
)

// All returns an iterator over index-value pairs, from 0 to Len-1.
func All[T any](s Slice) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		base := unsafe.Add(unsafe.Pointer(s), uintptr(s.dataOffset))
		es := uintptr(s.elemSize)
		for i := range int(s.len) {
			v := *(*T)(unsafe.Add(base, uintptr(i)*es))
			if !yield(i, v) {
				return
			}
		}
	}
}

// Values returns an iterator over values only.
func Values[T any](s Slice) iter.Seq[T] {
	return func(yield func(T) bool) {
		base := unsafe.Add(unsafe.Pointer(s), uintptr(s.dataOffset))
		es := uintptr(s.elemSize)
		for i := range int(s.len) {
			v := *(*T)(unsafe.Add(base, uintptr(i)*es))
			if !yield(v) {
				return
			}
		}
	}
}

// Backward returns an iterator over index-value pairs from Len-1 to 0.
func Backward[T any](s Slice) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		base := unsafe.Add(unsafe.Pointer(s), uintptr(s.dataOffset))
		es := uintptr(s.elemSize)
		for i := int(s.len) - 1; i >= 0; i-- {
			v := *(*T)(unsafe.Add(base, uintptr(i)*es))
			if !yield(i, v) {
				return
			}
		}
	}
}
