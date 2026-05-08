package mring_test

import (
	"testing"

	"github.com/gabstv/mmm"
	mring "github.com/gabstv/mmm/ring"
)

func BenchmarkPush1000_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	r := mring.New[float32](arena, 1000)
	b.ResetTimer()
	for b.Loop() {
		mring.Clear(r)
		for i := range 1000 {
			mring.Push[float32](r, float32(i))
		}
	}
}

func BenchmarkPush1000_GoSlice(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		ring := make([]float32, 0, 1000)
		for i := range 1000 {
			ring = append(ring, float32(i))
		}
	}
}

func BenchmarkPushOverwrite_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	r := mring.New[float32](arena, 64)
	b.ResetTimer()
	for b.Loop() {
		for range 1000 {
			mring.Push[float32](r, 1.0)
		}
	}
}

func BenchmarkPushOverwrite_GoManual(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		ring := make([]float32, 64)
		head := 0
		for range 1000 {
			ring[head] = 1.0
			head = (head + 1) % 64
		}
	}
}

type Vec3 struct{ X, Y, Z float32 }

func BenchmarkPushPopStruct_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	r := mring.New[Vec3](arena, 128)
	b.ResetTimer()
	for b.Loop() {
		mring.Clear(r)
		for i := range 128 {
			mring.Push[Vec3](r, Vec3{float32(i), float32(i), float32(i)})
		}
		for range 128 {
			mring.Pop[Vec3](r)
		}
	}
}

func BenchmarkIterAll_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	r := mring.New[int](arena, 1000)
	for i := range 1000 {
		mring.Push[int](r, i)
	}
	b.ResetTimer()
	var sink int
	for b.Loop() {
		for _, v := range mring.All[int](r) {
			sink = v
		}
	}
	_ = sink
}
