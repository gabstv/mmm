package mring

import (
	"iter"
	"unsafe"
)

// All returns an iterator over index-value pairs from oldest (0) to newest (Len-1).
func All[T any](r Ring) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		n := int(r.len)
		h := r.head
		m := r.mask
		base := unsafe.Pointer(r)
		do := uintptr(r.dataOffset)
		es := uintptr(r.elemSize)
		for i := range n {
			physical := (h + uint32(i)) & m
			v := *(*T)(unsafe.Add(base, do+uintptr(physical)*es))
			if !yield(i, v) {
				return
			}
		}
	}
}

// Values returns an iterator over values from oldest to newest.
func Values[T any](r Ring) iter.Seq[T] {
	return func(yield func(T) bool) {
		n := int(r.len)
		h := r.head
		m := r.mask
		base := unsafe.Pointer(r)
		do := uintptr(r.dataOffset)
		es := uintptr(r.elemSize)
		for i := range n {
			physical := (h + uint32(i)) & m
			v := *(*T)(unsafe.Add(base, do+uintptr(physical)*es))
			if !yield(v) {
				return
			}
		}
	}
}

// Backward returns an iterator over index-value pairs from newest (Len-1) to oldest (0).
func Backward[T any](r Ring) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		n := int(r.len)
		h := r.head
		m := r.mask
		base := unsafe.Pointer(r)
		do := uintptr(r.dataOffset)
		es := uintptr(r.elemSize)
		for i := n - 1; i >= 0; i-- {
			physical := (h + uint32(i)) & m
			v := *(*T)(unsafe.Add(base, do+uintptr(physical)*es))
			if !yield(i, v) {
				return
			}
		}
	}
}
