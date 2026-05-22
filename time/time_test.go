package mtime_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gabstv/mmm"
	mtime "github.com/gabstv/mmm/time"
)

func TestFrom(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	now := time.Date(2025, 6, 15, 10, 30, 0, 123456000, time.UTC)
	mt := mtime.From(arena, now)
	if mt == nil {
		t.Fatal("From returned nil")
	}
	got := mt.GoTime()
	if got.Unix() != now.Unix() {
		t.Fatalf("Unix = %d, want %d", got.Unix(), now.Unix())
	}
	if got.Nanosecond()/1000 != 123456 {
		t.Fatalf("usec = %d, want 123456", got.Nanosecond()/1000)
	}
}

func TestNow(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	before := time.Now()
	mt := mtime.Now(arena)
	after := time.Now()

	if mt == nil {
		t.Fatal("Now returned nil")
	}
	got := mt.GoTime()
	if got.Before(before.Truncate(time.Microsecond)) {
		t.Fatal("Now is before test start")
	}
	if got.After(after.Add(time.Microsecond)) {
		t.Fatal("Now is after test end")
	}
}

func TestNewTimestamp(t *testing.T) {
	ref := time.Date(2024, 1, 15, 13, 30, 0, 500000000, time.UTC)
	ts := mtime.NewTimestamp(ref)
	got := ts.GoTime()
	if got.Unix() != ref.Unix() {
		t.Fatalf("Unix = %d, want %d", got.Unix(), ref.Unix())
	}
	if got.Nanosecond()/1000 != 500000 {
		t.Fatalf("usec = %d, want 500000", got.Nanosecond()/1000)
	}
}

func TestSet(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	mt := mtime.New(arena)
	if !mt.IsZero() {
		t.Fatal("New should be zero")
	}
	ref := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)
	mt.Set(ref)
	got := mt.GoTime()
	if !got.Equal(ref) {
		t.Fatalf("GoTime = %v, want %v", got, ref)
	}
}

func TestIsZero(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	mt := mtime.New(arena)
	if !mt.IsZero() {
		t.Fatal("New should be zero")
	}
	mt.Set(time.Now())
	if mt.IsZero() {
		t.Fatal("should not be zero after Set")
	}
}

func TestUnix(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	ref := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	mt := mtime.From(arena, ref)
	if mt.Unix() != ref.Unix() {
		t.Fatalf("Unix = %d, want %d", mt.Unix(), ref.Unix())
	}
}

func TestUnixMicro(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	ref := time.Date(2025, 6, 1, 0, 0, 0, 123456000, time.UTC)
	mt := mtime.From(arena, ref)
	want := ref.Unix()*1_000_000 + 123456
	if mt.UnixMicro() != want {
		t.Fatalf("UnixMicro = %d, want %d", mt.UnixMicro(), want)
	}
}

func TestBeforeAfterEqual(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	t1 := mtime.From(arena, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	t2 := mtime.From(arena, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	t3 := mtime.From(arena, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	if !t1.Before(*t2) {
		t.Fatal("t1 should be before t2")
	}
	if t2.Before(*t1) {
		t.Fatal("t2 should not be before t1")
	}
	if !t2.After(*t1) {
		t.Fatal("t2 should be after t1")
	}
	if t1.After(*t2) {
		t.Fatal("t1 should not be after t2")
	}
	if !t1.Equal(*t3) {
		t.Fatal("t1 should equal t3")
	}
	if t1.Equal(*t2) {
		t.Fatal("t1 should not equal t2")
	}
}

func TestBeforeAfterSubMicrosecond(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	base := time.Date(2025, 1, 1, 0, 0, 1, 0, time.UTC)
	a := mtime.From(arena, base)
	b := mtime.From(arena, base.Add(time.Microsecond))

	if !a.Before(*b) {
		t.Fatal("a should be before b (1us apart)")
	}
	if a.Equal(*b) {
		t.Fatal("a should not equal b")
	}
}

func TestTimezoneRoundTrip(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skip("timezone data not available:", err)
	}
	ref := time.Date(2025, 6, 15, 14, 30, 0, 0, loc)
	mt := mtime.From(arena, ref)
	got := mt.GoTime()

	if got.Location().String() != "America/Sao_Paulo" {
		t.Fatalf("Location = %q, want %q", got.Location().String(), "America/Sao_Paulo")
	}
	if !got.Equal(ref) {
		t.Fatalf("GoTime = %v, want %v", got, ref)
	}
}

func TestTimezoneUTCAndLocal(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	utcTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	localTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.Local)

	mtUTC := mtime.From(arena, utcTime)
	mtLocal := mtime.From(arena, localTime)

	gotUTC := mtUTC.GoTime()
	gotLocal := mtLocal.GoTime()

	if gotUTC.Location().String() != "UTC" {
		t.Fatalf("UTC location = %q", gotUTC.Location().String())
	}
	if gotLocal.Location().String() != time.Local.String() {
		t.Fatalf("Local location = %q, want %q", gotLocal.Location().String(), time.Local.String())
	}
}

func TestMultipleTimezones(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	zones := []string{"America/New_York", "Europe/London", "Asia/Tokyo", "Australia/Sydney"}
	for _, name := range zones {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Skipf("timezone %q not available: %v", name, err)
		}
		ref := time.Date(2025, 6, 15, 12, 0, 0, 0, loc)
		mt := mtime.From(arena, ref)
		got := mt.GoTime()
		if got.Location().String() != name {
			t.Fatalf("Location = %q, want %q", got.Location().String(), name)
		}
	}
}

func TestMicrosecondTruncation(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	ref := time.Date(2025, 1, 1, 0, 0, 0, 123456789, time.UTC)
	mt := mtime.From(arena, ref)
	got := mt.GoTime()
	if got.Nanosecond() != 123456000 {
		t.Fatalf("Nanosecond = %d, want 123456000 (truncated from 123456789)", got.Nanosecond())
	}
}

func TestFree(t *testing.T) {
	gpa := mmm.NewGeneralPurposeAllocator(4096)

	t1 := mtime.From(gpa, time.Now())
	t2 := mtime.From(gpa, time.Now())

	if err := mtime.Free(gpa, t1); err != nil {
		t.Fatal(err)
	}
	if t2.IsZero() {
		t.Fatal("t2 should not be zero after freeing t1")
	}
	if err := mtime.Free(gpa, t2); err != nil {
		t.Fatal(err)
	}
}

func TestNewTimestampValueType(t *testing.T) {
	ref := time.Date(2025, 6, 1, 12, 0, 0, 500000000, time.UTC)
	ts := mtime.NewTimestamp(ref)

	got := ts.GoTime()
	if !got.Equal(ref.Truncate(time.Microsecond)) {
		t.Fatalf("GoTime = %v, want %v", got, ref.Truncate(time.Microsecond))
	}
}

func TestMultipleTimestampsContiguous(t *testing.T) {
	arena := mmm.NewArena(1 << 16)
	defer mmm.DestroyArena(&arena)

	stamps := make([]mtime.Time, 100)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range stamps {
		stamps[i] = mtime.From(arena, base.Add(time.Duration(i)*time.Hour))
		if stamps[i] == nil {
			t.Fatalf("allocation %d returned nil", i)
		}
	}
	for i, mt := range stamps {
		got := mt.GoTime()
		want := base.Add(time.Duration(i) * time.Hour)
		if !got.Equal(want) {
			t.Fatalf("stamp %d = %v, want %v", i, got, want)
		}
	}
}

// --- JSON tests ---

func TestMarshalJSONRFC3339(t *testing.T) {
	mtime.SetJSONFormat(mtime.FormatRFC3339)
	defer mtime.SetJSONFormat(mtime.FormatRFC3339)

	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	ref := time.Date(2025, 6, 15, 10, 30, 0, 123456000, time.UTC)
	mt := mtime.From(arena, ref)

	data, err := json.Marshal(mt)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(ref.Truncate(time.Microsecond))
	if string(data) != string(want) {
		t.Fatalf("Marshal = %s, want %s", data, want)
	}
}

func TestMarshalJSONUnixMicro(t *testing.T) {
	mtime.SetJSONFormat(mtime.FormatUnixMicro)
	defer mtime.SetJSONFormat(mtime.FormatRFC3339)

	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	ref := time.Date(2025, 6, 15, 10, 30, 0, 123456000, time.UTC)
	mt := mtime.From(arena, ref)

	data, err := json.Marshal(mt)
	if err != nil {
		t.Fatal(err)
	}
	var got int64
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := ref.Unix()*1_000_000 + 123456
	if got != want {
		t.Fatalf("UnixMicro = %d, want %d", got, want)
	}
}

func TestMarshalJSONUnixMilli(t *testing.T) {
	mtime.SetJSONFormat(mtime.FormatUnixMilli)
	defer mtime.SetJSONFormat(mtime.FormatRFC3339)

	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	ref := time.Date(2025, 6, 15, 10, 30, 0, 123000000, time.UTC)
	mt := mtime.From(arena, ref)

	data, err := json.Marshal(mt)
	if err != nil {
		t.Fatal(err)
	}
	var got int64
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := ref.Unix()*1000 + 123
	if got != want {
		t.Fatalf("UnixMilli = %d, want %d", got, want)
	}
}

func TestUnmarshalJSONRFC3339(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	mt := mtime.New(arena)
	input := `"2025-06-15T10:30:00.123456Z"`
	if err := json.Unmarshal([]byte(input), mt); err != nil {
		t.Fatal(err)
	}
	got := mt.GoTime()
	want := time.Date(2025, 6, 15, 10, 30, 0, 123456000, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("after unmarshal: %v, want %v", got, want)
	}
}

func TestUnmarshalJSONNumber(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	ref := time.Date(2025, 6, 15, 10, 30, 0, 123456000, time.UTC)

	tests := []struct {
		name     string
		input    string
		wantSec  int64
		wantUsec int
	}{
		{
			name:     "unix_micro",
			input:    fmt.Sprintf("%d", ref.Unix()*1_000_000+123456),
			wantSec:  ref.Unix(),
			wantUsec: 123456,
		},
		{
			name:     "unix_milli",
			input:    fmt.Sprintf("%d", ref.Unix()*1000+123),
			wantSec:  ref.Unix(),
			wantUsec: 123000,
		},
		{
			name:     "unix_seconds",
			input:    fmt.Sprintf("%d", ref.Unix()),
			wantSec:  ref.Unix(),
			wantUsec: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := mtime.New(arena)
			if err := json.Unmarshal([]byte(tt.input), mt); err != nil {
				t.Fatal(err)
			}
			if mt.Unix() != tt.wantSec {
				t.Fatalf("Unix = %d, want %d", mt.Unix(), tt.wantSec)
			}
			got := mt.GoTime()
			if got.Nanosecond()/1000 != tt.wantUsec {
				t.Fatalf("usec = %d, want %d", got.Nanosecond()/1000, tt.wantUsec)
			}
		})
	}
}

func TestUnmarshalJSONNull(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	mt := mtime.From(arena, time.Now())
	if err := json.Unmarshal([]byte("null"), mt); err != nil {
		t.Fatal(err)
	}
	if !mt.IsZero() {
		t.Fatal("should be zero after null unmarshal")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	mtime.SetJSONFormat(mtime.FormatRFC3339)
	defer mtime.SetJSONFormat(mtime.FormatRFC3339)

	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	ref := time.Date(2025, 6, 15, 10, 30, 0, 123456000, time.UTC)
	mt := mtime.From(arena, ref)

	data, err := json.Marshal(mt)
	if err != nil {
		t.Fatal(err)
	}

	mt2 := mtime.New(arena)
	if err := json.Unmarshal(data, mt2); err != nil {
		t.Fatal(err)
	}
	if !mt.Equal(*mt2) {
		t.Fatalf("round trip: %v != %v", mt.GoTime(), mt2.GoTime())
	}
}

func TestJSONRoundTripAllFormats(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	ref := time.Date(2025, 6, 15, 10, 30, 0, 123000000, time.UTC)

	for _, f := range []mtime.JSONFormat{mtime.FormatRFC3339, mtime.FormatUnixMicro, mtime.FormatUnixMilli} {
		mtime.SetJSONFormat(f)
		mt := mtime.From(arena, ref)
		data, err := json.Marshal(mt)
		if err != nil {
			t.Fatalf("format %d: marshal: %v", f, err)
		}
		mt2 := mtime.New(arena)
		if err := json.Unmarshal(data, mt2); err != nil {
			t.Fatalf("format %d: unmarshal %s: %v", f, data, err)
		}
		if mt.Unix() != mt2.Unix() {
			t.Fatalf("format %d: seconds mismatch: %d != %d", f, mt.Unix(), mt2.Unix())
		}
		if mt.UnixMicro()/1000 != mt2.UnixMicro()/1000 {
			t.Fatalf("format %d: millis mismatch: %d != %d", f, mt.UnixMicro()/1000, mt2.UnixMicro()/1000)
		}
	}
	mtime.SetJSONFormat(mtime.FormatRFC3339)
}

func TestJSONInStruct(t *testing.T) {
	mtime.SetJSONFormat(mtime.FormatRFC3339)
	defer mtime.SetJSONFormat(mtime.FormatRFC3339)

	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	type Event struct {
		Name string     `json:"name"`
		At   mtime.Time `json:"at"`
	}

	ref := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	e := Event{Name: "test", At: mtime.From(arena, ref)}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}

	e2 := Event{At: mtime.New(arena)}
	if err := json.Unmarshal(data, &e2); err != nil {
		t.Fatal(err)
	}
	if e2.Name != "test" {
		t.Fatalf("Name = %q, want %q", e2.Name, "test")
	}
	if !e.At.Equal(*e2.At) {
		t.Fatalf("At = %v, want %v", e2.At.GoTime(), e.At.GoTime())
	}
}

// --- Negative / pre-epoch timestamps ---

func TestNegativeTimestamp(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	ref := time.Date(1969, 12, 31, 23, 59, 59, 500000000, time.UTC)
	mt := mtime.From(arena, ref)
	got := mt.GoTime()
	if !got.Equal(ref.Truncate(time.Microsecond)) {
		t.Fatalf("GoTime = %v, want %v", got, ref.Truncate(time.Microsecond))
	}
}

func TestPreEpochRoundTrip(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	dates := []time.Time{
		time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(1969, 7, 20, 20, 17, 0, 0, time.UTC), // Moon landing
		time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),       // Go zero time (year 1)
	}
	for _, ref := range dates {
		mt := mtime.From(arena, ref)
		got := mt.GoTime()
		if !got.Equal(ref.Truncate(time.Microsecond)) {
			t.Fatalf("ref=%v: GoTime = %v, want %v", ref, got, ref.Truncate(time.Microsecond))
		}
	}
}

func TestNegativeTimestampJSONRoundTrip(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	ref := time.Date(1969, 6, 15, 10, 30, 0, 123000000, time.UTC)

	for _, f := range []mtime.JSONFormat{mtime.FormatRFC3339, mtime.FormatUnixMicro, mtime.FormatUnixMilli} {
		mtime.SetJSONFormat(f)
		mt := mtime.From(arena, ref)
		data, err := json.Marshal(mt)
		if err != nil {
			t.Fatalf("format %d: marshal: %v", f, err)
		}
		mt2 := mtime.New(arena)
		if err := json.Unmarshal(data, mt2); err != nil {
			t.Fatalf("format %d: unmarshal %s: %v", f, data, err)
		}
		if mt.Unix() != mt2.Unix() {
			t.Fatalf("format %d: seconds mismatch: %d != %d (data=%s)", f, mt.Unix(), mt2.Unix(), data)
		}
		if mt.UnixMicro()/1000 != mt2.UnixMicro()/1000 {
			t.Fatalf("format %d: millis mismatch: %d != %d (data=%s)", f, mt.UnixMicro()/1000, mt2.UnixMicro()/1000, data)
		}
	}
	mtime.SetJSONFormat(mtime.FormatRFC3339)
}

// --- Edge cases ---

func TestUnmarshalJSONInvalid(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	invalid := []string{
		`"not-a-date"`,
		`true`,
		`"2025-13-01T00:00:00Z"`, // invalid month
	}
	for _, input := range invalid {
		mt := mtime.New(arena)
		if err := json.Unmarshal([]byte(input), mt); err == nil {
			t.Fatalf("expected error for input %s", input)
		}
	}
}

func TestMarshalUnmarshalZeroValue(t *testing.T) {
	mtime.SetJSONFormat(mtime.FormatRFC3339)
	defer mtime.SetJSONFormat(mtime.FormatRFC3339)

	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	mt := mtime.New(arena) // zero value
	data, err := json.Marshal(mt)
	if err != nil {
		t.Fatal(err)
	}

	mt2 := mtime.New(arena)
	if err := json.Unmarshal(data, mt2); err != nil {
		t.Fatal(err)
	}
	if !mt2.IsZero() {
		t.Fatalf("expected zero after round-tripping zero, got %v", mt2.GoTime())
	}
}

func TestFixedZone(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	loc := time.FixedZone("Custom+05:30", 5*3600+30*60)
	ref := time.Date(2025, 6, 15, 12, 0, 0, 0, loc)
	mt := mtime.From(arena, ref)
	got := mt.GoTime()

	if got.Location().String() != "Custom+05:30" {
		t.Fatalf("Location = %q, want %q", got.Location().String(), "Custom+05:30")
	}
	if !got.Equal(ref) {
		t.Fatalf("GoTime = %v, want %v", got, ref)
	}
}

func TestNewTimestampWithTimezone(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("timezone data not available:", err)
	}
	ref := time.Date(2025, 6, 15, 14, 0, 0, 0, loc)
	ts := mtime.NewTimestamp(ref)
	got := ts.GoTime()
	if got.Location().String() != "Europe/Berlin" {
		t.Fatalf("Location = %q, want %q", got.Location().String(), "Europe/Berlin")
	}
}

func TestIsZeroWithNonUTCZone(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("timezone data not available:", err)
	}
	// Epoch instant but in a non-UTC zone
	ref := time.Unix(0, 0).In(loc)
	mt := mtime.From(arena, ref)
	// sec=0, usec=0, but zoneID != 0 → not the zero Timestamp
	if mt.IsZero() {
		t.Fatal("epoch in non-UTC zone should not be IsZero")
	}
}
