package mslices

type header struct {
	len        uint32
	cap        uint32
	elemSize   uint32
	dataOffset uint32
}

type Slice = *header
