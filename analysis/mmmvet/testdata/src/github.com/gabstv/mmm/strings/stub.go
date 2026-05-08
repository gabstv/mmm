package mstrings

type header struct {
	len uint32
	cap uint32
}

type String = *header
