package mtime

type Timestamp struct {
	sec    int64
	usec   uint32
	zoneID uint16
	_      [2]byte
}

type Time = *Timestamp
