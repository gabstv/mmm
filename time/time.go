// Package mtime provides an arena-allocated time representation with
// microsecond precision. The Timestamp value type is 16 bytes with zero
// GC pointers, making it safe to store directly in arena memory, inside
// mslices.Slice[Timestamp], or mring.Ring[Timestamp] without pinning.
//
// Timezone identity is preserved through a package-level registry that
// maps compact uint16 indices to *time.Location values. Round-tripping
// through GoTime() reconstructs the full time.Time with the original
// IANA zone (including DST rules), not just a fixed UTC offset.
//
// Precision: microseconds. Nanoseconds from time.Time are truncated on
// storage. This matches database timestamp precision (PostgreSQL, MySQL).
//
// JSON serialization format is controlled by SetJSONFormat (default:
// RFC 3339). When unmarshaling numeric JSON values, the current format
// setting determines interpretation. When the format is RFC 3339
// (default), a magnitude-based heuristic auto-detects the unit for
// numeric values, which is reliable for timestamps after 2001-09-09
// but ambiguous for earlier dates or pre-epoch values.
package mtime

import (
	"strconv"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/gabstv/mmm"
)

// Timestamp is a 16-byte, pointer-free representation of a point in time.
// It stores Unix seconds, microseconds, and a timezone registry index.
// Safe for direct storage in arena memory without GC pinning.
type Timestamp struct {
	sec    int64
	usec   uint32
	zoneID uint16
	_      [2]byte
}

// Time is an arena-allocated pointer to a Timestamp.
type Time = *Timestamp

const timestampSize = int(unsafe.Sizeof(Timestamp{}))
const timestampAlign = int(unsafe.Alignof(Timestamp{}))

// JSONFormat controls how Timestamp values are serialized to JSON.
type JSONFormat int32

const (
	// FormatRFC3339 serializes as an RFC 3339 string (default).
	// Microsecond precision is included when nonzero.
	FormatRFC3339 JSONFormat = iota
	// FormatUnixMicro serializes as a JSON number (Unix microseconds).
	FormatUnixMicro
	// FormatUnixMilli serializes as a JSON number (Unix milliseconds).
	FormatUnixMilli
)

var jsonFormat atomic.Int32

// SetJSONFormat sets the global JSON serialization format for Timestamp.
// This affects MarshalJSON output and how UnmarshalJSON interprets numeric
// values. RFC 3339 strings are always accepted regardless of setting.
func SetJSONFormat(f JSONFormat) {
	jsonFormat.Store(int32(f))
}

// New allocates a zero-valued Timestamp in arena memory.
// The zero value represents 1970-01-01T00:00:00Z.
// Returns nil if the allocator cannot satisfy the request.
func New(a mmm.Allocator) Time {
	ptr := mmm.RawAlloc(a, timestampSize, timestampAlign)
	if ptr == nil {
		return nil
	}
	return (Time)(ptr)
}

// Now allocates a Timestamp set to the current time.
// Returns nil if the allocator cannot satisfy the request.
func Now(a mmm.Allocator) Time {
	t := New(a)
	if t == nil {
		return nil
	}
	t.Set(time.Now())
	return t
}

// From allocates a Timestamp and sets it from a time.Time.
// Returns nil if the allocator cannot satisfy the request.
func From(a mmm.Allocator, v time.Time) Time {
	t := New(a)
	if t == nil {
		return nil
	}
	t.Set(v)
	return t
}

// NewTimestamp creates a Timestamp value from a time.Time.
// No arena allocation; the result lives on the stack or wherever the caller stores it.
func NewTimestamp(v time.Time) Timestamp {
	var ts Timestamp
	ts.Set(v)
	return ts
}

// Free releases a Time allocation from the allocator.
func Free(a mmm.Allocator, t Time) error {
	return mmm.RawFree(a, unsafe.Pointer(t))
}

// Set updates the Timestamp in place from a time.Time.
func (ts *Timestamp) Set(v time.Time) {
	ts.sec = v.Unix()
	ts.usec = uint32(v.Nanosecond() / 1000)
	ts.zoneID = registerLocation(v.Location())
}

// GoTime reconstructs a time.Time from the Timestamp.
func (ts *Timestamp) GoTime() time.Time {
	loc := resolveLocation(ts.zoneID)
	return time.Unix(ts.sec, int64(ts.usec)*1000).In(loc)
}

// Unix returns the number of seconds elapsed since the Unix epoch.
func (ts *Timestamp) Unix() int64 {
	return ts.sec
}

// UnixMicro returns the number of microseconds elapsed since the Unix epoch.
func (ts *Timestamp) UnixMicro() int64 {
	return ts.sec*1_000_000 + int64(ts.usec)
}

// IsZero reports whether the Timestamp represents the zero value
// (1970-01-01T00:00:00 UTC).
func (ts *Timestamp) IsZero() bool {
	return ts.sec == 0 && ts.usec == 0 && ts.zoneID == 0
}

// Before reports whether ts is before other.
func (ts *Timestamp) Before(other Timestamp) bool {
	if ts.sec != other.sec {
		return ts.sec < other.sec
	}
	return ts.usec < other.usec
}

// After reports whether ts is after other.
func (ts *Timestamp) After(other Timestamp) bool {
	if ts.sec != other.sec {
		return ts.sec > other.sec
	}
	return ts.usec > other.usec
}

// Equal reports whether ts and other represent the same point in time.
// Like time.Time.Equal, this compares the instant, not the timezone.
func (ts *Timestamp) Equal(other Timestamp) bool {
	return ts.sec == other.sec && ts.usec == other.usec
}

// MarshalJSON implements json.Marshaler. The output format is controlled by
// SetJSONFormat (default: RFC 3339 with microsecond precision).
func (ts *Timestamp) MarshalJSON() ([]byte, error) {
	switch JSONFormat(jsonFormat.Load()) {
	case FormatUnixMicro:
		return strconv.AppendInt(nil, ts.UnixMicro(), 10), nil
	case FormatUnixMilli:
		ms := ts.sec*1000 + int64(ts.usec)/1000
		return strconv.AppendInt(nil, ms, 10), nil
	default:
		return ts.GoTime().MarshalJSON()
	}
}

// UnmarshalJSON implements json.Unmarshaler. RFC 3339 strings are always
// accepted. For numeric JSON values, the current JSONFormat setting
// determines interpretation: FormatUnixMicro reads microseconds,
// FormatUnixMilli reads milliseconds, FormatRFC3339 (default) uses a
// magnitude-based heuristic that is reliable for timestamps after
// 2001-09-09 but ambiguous for earlier dates.
func (ts *Timestamp) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		ts.sec = 0
		ts.usec = 0
		ts.zoneID = 0
		return nil
	}

	n := len(data)

	// JSON string → RFC 3339
	if n >= 2 && data[0] == '"' && data[n-1] == '"' {
		t, err := time.Parse(time.RFC3339Nano, string(data[1:n-1]))
		if err != nil {
			return err
		}
		ts.Set(t)
		return nil
	}

	// JSON number → Unix timestamp
	v, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}

	// When the format is explicitly set, use it directly.
	// When the format is RFC 3339 (default), fall back to a magnitude heuristic.
	format := JSONFormat(jsonFormat.Load())
	switch {
	case format == FormatUnixMicro:
		setFromMicro(ts, v)
	case format == FormatUnixMilli:
		setFromMilli(ts, v)
	case v > 1e15 || v < -1e15:
		setFromMicro(ts, v)
	case v > 1e12 || v < -1e12:
		setFromMilli(ts, v)
	default:
		ts.sec = v
		ts.usec = 0
	}
	ts.zoneID = 0
	return nil
}

// setFromMicro decomposes a Unix-microsecond value into sec + usec
// using floor division so that usec is always in [0, 999999].
func setFromMicro(ts *Timestamp, v int64) {
	ts.sec = v / 1_000_000
	rem := v % 1_000_000
	if rem < 0 {
		ts.sec--
		rem += 1_000_000
	}
	ts.usec = uint32(rem)
}

// setFromMilli decomposes a Unix-millisecond value into sec + usec
// using floor division so that usec is always in [0, 999999].
func setFromMilli(ts *Timestamp, v int64) {
	ts.sec = v / 1000
	rem := v % 1000
	if rem < 0 {
		ts.sec--
		rem += 1000
	}
	ts.usec = uint32(rem) * 1000
}
