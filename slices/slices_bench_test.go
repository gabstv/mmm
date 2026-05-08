package mslices_test

import (
	"testing"

	"github.com/gabstv/mmm"
	mslices "github.com/gabstv/mmm/slices"
)

func BenchmarkAppend1000_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	b.ResetTimer()
	for b.Loop() {
		arena.Reset()
		s := mslices.New[float32](arena, 1000)
		for i := range 1000 {
			mslices.Append[float32](s, float32(i))
		}
	}
}

func BenchmarkAppend1000_GoSlice(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		s := make([]float32, 0, 1000)
		for i := range 1000 {
			s = append(s, float32(i))
		}
	}
}

func BenchmarkFrom1000_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	src := make([]float64, 1000)
	for i := range src {
		src[i] = float64(i)
	}
	b.ResetTimer()
	for b.Loop() {
		arena.Reset()
		_ = mslices.From[float64](arena, src)
	}
}

func BenchmarkFrom1000_GoSlice(b *testing.B) {
	src := make([]float64, 1000)
	for i := range src {
		src[i] = float64(i)
	}
	b.ResetTimer()
	var sink []float64
	for b.Loop() {
		dst := make([]float64, 1000)
		copy(dst, src)
		sink = dst
	}
	_ = sink
}

func BenchmarkGet1000_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	s := mslices.From[float32](arena, make([]float32, 1000))
	b.ResetTimer()
	var sink float32
	for b.Loop() {
		for i := range 1000 {
			sink = *mslices.Get[float32](s, i)
		}
	}
	_ = sink
}

func BenchmarkGet1000_GoSlice(b *testing.B) {
	s := make([]float32, 1000)
	b.ResetTimer()
	var sink float32
	for b.Loop() {
		for i := range 1000 {
			sink = s[i]
		}
	}
	_ = sink
}

func BenchmarkStructAppend_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	b.ResetTimer()
	for b.Loop() {
		arena.Reset()
		s := mslices.New[Vec3](arena, 1000)
		for i := range 1000 {
			mslices.Append[Vec3](s, Vec3{float32(i), float32(i), float32(i)})
		}
	}
}

func BenchmarkStructAppend_GoSlice(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		s := make([]Vec3, 0, 1000)
		for i := range 1000 {
			s = append(s, Vec3{float32(i), float32(i), float32(i)})
		}
	}
}
