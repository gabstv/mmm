package mtime_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gabstv/mmm"
	mtime "github.com/gabstv/mmm/time"
)

func BenchmarkFrom_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	ref := time.Now()
	b.ResetTimer()
	for b.Loop() {
		arena.Reset()
		_ = mtime.From(arena, ref)
	}
}

func BenchmarkFrom_GoTime(b *testing.B) {
	ref := time.Now()
	b.ResetTimer()
	var sink time.Time
	for b.Loop() {
		sink = ref
	}
	_ = sink
}

func BenchmarkNewTimestamp(b *testing.B) {
	ref := time.Now()
	b.ResetTimer()
	var sink mtime.Timestamp
	for b.Loop() {
		sink = mtime.NewTimestamp(ref)
	}
	_ = sink
}

func BenchmarkGoTime(b *testing.B) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)
	mt := mtime.From(arena, time.Now())
	b.ResetTimer()
	var sink time.Time
	for b.Loop() {
		sink = mt.GoTime()
	}
	_ = sink
}

func BenchmarkFrom1000_Arena(b *testing.B) {
	arena := mmm.NewArena(1 << 20)
	ref := time.Now()
	b.ResetTimer()
	for b.Loop() {
		arena.Reset()
		for range 1000 {
			_ = mtime.From(arena, ref)
		}
	}
}

func BenchmarkFrom1000_GoHeap(b *testing.B) {
	ref := time.Now()
	b.ResetTimer()
	for b.Loop() {
		for range 1000 {
			t := new(time.Time)
			*t = ref
			_ = t
		}
	}
}

func BenchmarkMarshalJSON_RFC3339(b *testing.B) {
	mtime.SetJSONFormat(mtime.FormatRFC3339)
	defer mtime.SetJSONFormat(mtime.FormatRFC3339)
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)
	mt := mtime.From(arena, time.Now())
	b.ResetTimer()
	for b.Loop() {
		_, _ = json.Marshal(mt)
	}
}

func BenchmarkMarshalJSON_UnixMicro(b *testing.B) {
	mtime.SetJSONFormat(mtime.FormatUnixMicro)
	defer mtime.SetJSONFormat(mtime.FormatRFC3339)
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)
	mt := mtime.From(arena, time.Now())
	b.ResetTimer()
	for b.Loop() {
		_, _ = json.Marshal(mt)
	}
}

func BenchmarkUnmarshalJSON_RFC3339(b *testing.B) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)
	mt := mtime.New(arena)
	data := []byte(`"2025-06-15T10:30:00.123456Z"`)
	b.ResetTimer()
	for b.Loop() {
		_ = json.Unmarshal(data, mt)
	}
}
