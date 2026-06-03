package mstrings_test

import (
	"testing"

	"github.com/gabstv/mmm"
	mstrings "github.com/gabstv/mmm/strings"
)

// fakeReader implements mstrings.ZReader by decoding into a real arena.
type fakeReader struct {
	arena mmm.Allocator
	value string
	err   error
}

func (r *fakeReader) ReadStringIntoArena() (mstrings.String, error) {
	if r.err != nil {
		return nil, r.err
	}
	return mstrings.From(r.arena, r.value), nil
}

// fakeWriter implements mstrings.ZWriter, capturing the quoted string call.
type fakeWriter struct {
	got string
}

func (w *fakeWriter) WriteQuotedString(s string) {
	w.got = s
}

func TestZRead(t *testing.T) {
	arena := mmm.NewArena(256)
	defer mmm.DestroyArena(&arena)

	r := &fakeReader{arena: arena, value: "hello"}
	s, err := mstrings.ZRead(r)
	if err != nil {
		t.Fatalf("ZRead error: %v", err)
	}
	if !mstrings.EqualString(s, "hello") {
		t.Fatalf("ZRead = %q, want %q", mstrings.GoString(s), "hello")
	}
}

func TestZReadError(t *testing.T) {
	r := &fakeReader{err: errSentinel}
	if _, err := mstrings.ZRead(r); err != errSentinel {
		t.Fatalf("ZRead err = %v, want sentinel", err)
	}
}

var errSentinel = errString("boom")

type errString string

func (e errString) Error() string { return string(e) }

func TestZWrite(t *testing.T) {
	arena := mmm.NewArena(256)
	defer mmm.DestroyArena(&arena)

	s := mstrings.From(arena, "world")
	w := &fakeWriter{}
	mstrings.ZWrite(w, s)
	if w.got != "world" {
		t.Fatalf("ZWrite passed %q, want %q", w.got, "world")
	}
}
