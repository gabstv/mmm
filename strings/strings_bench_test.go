package mstrings_test

import (
	"testing"

	"github.com/gabstv/mmm"
	mstrings "github.com/gabstv/mmm/strings"
)

func BenchmarkFrom_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20) // 1MB
	src := "the quick brown fox jumps over the lazy dog"
	b.ResetTimer()
	for b.Loop() {
		arena.Reset()
		_ = mstrings.From(arena, src)
	}
}

func BenchmarkFrom_GoString(b *testing.B) {
	src := []byte("the quick brown fox jumps over the lazy dog")
	b.ResetTimer()
	var sink string
	for b.Loop() {
		sink = string(src)
	}
	_ = sink
}

func BenchmarkFrom1000_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	src := "the quick brown fox jumps over the lazy dog"
	b.ResetTimer()
	for b.Loop() {
		arena.Reset()
		for range 1000 {
			_ = mstrings.From(arena, src)
		}
	}
}

func BenchmarkFrom1000_GoString(b *testing.B) {
	src := []byte("the quick brown fox jumps over the lazy dog")
	b.ResetTimer()
	var sink string
	for b.Loop() {
		for range 1000 {
			sink = string(src)
		}
	}
	_ = sink
}

func BenchmarkAppend_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	b.ResetTimer()
	for b.Loop() {
		arena.Reset()
		s := mstrings.New(arena, 256)
		for range 10 {
			mstrings.Append(s, "hello world! ")
		}
	}
}

func BenchmarkAppend_GoConcat(b *testing.B) {
	b.ResetTimer()
	var sink string
	for b.Loop() {
		s := ""
		for range 10 {
			s += "hello world! "
		}
		sink = s
	}
	_ = sink
}
