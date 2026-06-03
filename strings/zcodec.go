package mstrings

// ZReader is the minimal subset of zajson's *Reader that mstrings needs to
// decode a String straight into the arena. *zajson.Reader satisfies it
// structurally, so zajson never has to import per-element decode logic and
// mstrings never imports zajson.
type ZReader interface {
	ReadStringIntoArena() (String, error)
}

// ZWriter is the minimal subset of zajson's *Writer that mstrings needs to
// encode a String. *zajson.Writer satisfies it structurally.
type ZWriter interface {
	WriteQuotedString(string)
}

// ZRead decodes a JSON string from r directly into the reader's arena and
// returns the resulting String.
//
// It is a free function (not a method) because String is a pointer alias: a
// value receiver could not reassign the caller's variable, and there is no
// existing String to mutate during a fresh decode. zajson codegen calls this
// to read an mstrings.String element.
func ZRead(r ZReader) (String, error) {
	return r.ReadStringIntoArena()
}

// ZWrite encodes s as a quoted JSON string into w. zajson codegen calls this
// to write an mstrings.String element.
func ZWrite(w ZWriter, s String) {
	w.WriteQuotedString(GoString(s))
}
