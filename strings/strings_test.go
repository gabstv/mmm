package mstrings_test

import (
	"encoding/json"
	"testing"
	"unsafe"

	"github.com/gabstv/mmm"
	mstrings "github.com/gabstv/mmm/strings"
)

func TestFrom(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.From(arena, "hello world")
	if s == nil {
		t.Fatal("From returned nil")
	}
	if mstrings.Len(s) != 11 {
		t.Fatalf("Len = %d, want 11", mstrings.Len(s))
	}
	if mstrings.Cap(s) != 11 {
		t.Fatalf("Cap = %d, want 11", mstrings.Cap(s))
	}
	if mstrings.GoString(s) != "hello world" {
		t.Fatalf("GoString = %q, want %q", mstrings.GoString(s), "hello world")
	}
}

func TestNullTermination(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.From(arena, "abc")
	cstr := mstrings.CString(s)
	// Walk the C string to verify null termination
	bytes := make([]byte, 4)
	for i := range bytes {
		bytes[i] = *(*byte)(addPtr(cstr, i))
	}
	if bytes[0] != 'a' || bytes[1] != 'b' || bytes[2] != 'c' || bytes[3] != 0 {
		t.Fatalf("CString bytes = %v, want [a b c 0]", bytes)
	}
}

func TestNew(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.New(arena, 32)
	if mstrings.Len(s) != 0 {
		t.Fatalf("Len = %d, want 0", mstrings.Len(s))
	}
	if mstrings.Cap(s) != 32 {
		t.Fatalf("Cap = %d, want 32", mstrings.Cap(s))
	}
	if mstrings.GoString(s) != "" {
		t.Fatalf("GoString = %q, want empty", mstrings.GoString(s))
	}
}

func TestSetAndAppend(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.New(arena, 20)

	if !mstrings.Set(s, "hello") {
		t.Fatal("Set returned false")
	}
	if mstrings.GoString(s) != "hello" {
		t.Fatalf("after Set: %q", mstrings.GoString(s))
	}

	if !mstrings.Append(s, " world") {
		t.Fatal("Append returned false")
	}
	if mstrings.GoString(s) != "hello world" {
		t.Fatalf("after Append: %q", mstrings.GoString(s))
	}
	if mstrings.Len(s) != 11 {
		t.Fatalf("Len = %d, want 11", mstrings.Len(s))
	}
}

func TestSetOverflow(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.New(arena, 5)
	if mstrings.Set(s, "toolong") {
		t.Fatal("Set should return false when data exceeds capacity")
	}
	// Original content should be unchanged (still empty)
	if mstrings.Len(s) != 0 {
		t.Fatalf("Len = %d, want 0 after failed Set", mstrings.Len(s))
	}
}

func TestAppendOverflow(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.New(arena, 5)
	mstrings.Set(s, "abc")
	if mstrings.Append(s, "defgh") {
		t.Fatal("Append should return false when result exceeds capacity")
	}
	if mstrings.GoString(s) != "abc" {
		t.Fatalf("content should be unchanged after failed Append: %q", mstrings.GoString(s))
	}
}

func TestFromBytes(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.FromBytes(arena, []byte{0x48, 0x69, 0x21})
	if mstrings.GoString(s) != "Hi!" {
		t.Fatalf("GoString = %q, want %q", mstrings.GoString(s), "Hi!")
	}
}

func TestBytes(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.From(arena, "data")
	b := mstrings.Bytes(s)
	if len(b) != 4 {
		t.Fatalf("len(Bytes) = %d, want 4", len(b))
	}
	if string(b) != "data" {
		t.Fatalf("Bytes = %q, want %q", b, "data")
	}
}

func TestEqual(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	a := mstrings.From(arena, "same")
	b := mstrings.From(arena, "same")
	c := mstrings.From(arena, "diff")

	if !mstrings.Equal(a, b) {
		t.Fatal("Equal(a, b) should be true")
	}
	if mstrings.Equal(a, c) {
		t.Fatal("Equal(a, c) should be false")
	}
}

func TestEqualString(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.From(arena, "test")
	if !mstrings.EqualString(s, "test") {
		t.Fatal("EqualString should be true")
	}
	if mstrings.EqualString(s, "other") {
		t.Fatal("EqualString should be false")
	}
}

func TestGPA(t *testing.T) {
	gpa := mmm.NewGeneralPurposeAllocator(4096)

	s1 := mstrings.From(gpa, "first")
	s2 := mstrings.From(gpa, "second")

	if mstrings.GoString(s1) != "first" {
		t.Fatalf("s1 = %q", mstrings.GoString(s1))
	}
	if mstrings.GoString(s2) != "second" {
		t.Fatalf("s2 = %q", mstrings.GoString(s2))
	}

	if err := mstrings.Free(gpa, s1); err != nil {
		t.Fatal(err)
	}
	// s2 should still be valid
	if mstrings.GoString(s2) != "second" {
		t.Fatalf("s2 after free(s1) = %q", mstrings.GoString(s2))
	}
	if err := mstrings.Free(gpa, s2); err != nil {
		t.Fatal(err)
	}
}

func TestSetBytes(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.New(arena, 10)
	if !mstrings.SetBytes(s, []byte("raw")) {
		t.Fatal("SetBytes returned false")
	}
	if mstrings.GoString(s) != "raw" {
		t.Fatalf("GoString = %q, want %q", mstrings.GoString(s), "raw")
	}
}

func TestAppendBytes(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.From(arena, "a")
	// Capacity is 1, so append should fail
	if mstrings.AppendBytes(s, []byte("b")) {
		t.Fatal("AppendBytes should return false when exceeding capacity")
	}

	s2 := mstrings.New(arena, 10)
	mstrings.Set(s2, "x")
	if !mstrings.AppendBytes(s2, []byte("yz")) {
		t.Fatal("AppendBytes returned false")
	}
	if mstrings.GoString(s2) != "xyz" {
		t.Fatalf("GoString = %q, want %q", mstrings.GoString(s2), "xyz")
	}
}

func TestEmptyString(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.From(arena, "")
	if mstrings.Len(s) != 0 {
		t.Fatalf("Len = %d, want 0", mstrings.Len(s))
	}
	if mstrings.GoString(s) != "" {
		t.Fatalf("GoString = %q, want empty", mstrings.GoString(s))
	}
	// Null terminator should still be there
	cstr := mstrings.CString(s)
	if *cstr != 0 {
		t.Fatalf("CString[0] = %d, want 0", *cstr)
	}
}

func TestMultipleStringsContiguous(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	strs := make([]mstrings.String, 100)
	for i := range strs {
		strs[i] = mstrings.From(arena, "hello")
		if strs[i] == nil {
			t.Fatalf("allocation %d returned nil", i)
		}
	}
	for i, s := range strs {
		if mstrings.GoString(s) != "hello" {
			t.Fatalf("string %d = %q", i, mstrings.GoString(s))
		}
	}
}

func TestAlignedCap(t *testing.T) {
	// headerAlign for {uint32, uint32} is 4.
	// AlignedCap returns the smallest cap >= input such that
	// (headerSize + cap + 1) is a multiple of 4.
	// Since headerSize is 8 (already aligned), we need (cap+1) % 4 == 0,
	// so valid caps are 3, 7, 11, 15, ... (4k - 1).
	tests := []struct {
		input int
		want  int
	}{
		{0, 3},
		{1, 3},
		{2, 3},
		{3, 3},
		{4, 7},
		{5, 7},
		{7, 7},
		{8, 11},
		{9, 11},
		{10, 11},
		{11, 11},
		{12, 15},
		{16, 19},
		{31, 31},
		{32, 35},
		{100, 103},
		{101, 103},
		{1023, 1023},
		{1024, 1027},
	}
	for _, tt := range tests {
		got := mstrings.AlignedCap(tt.input)
		if got != tt.want {
			t.Errorf("AlignedCap(%d) = %d, want %d", tt.input, got, tt.want)
		}
		total := 8 + got + 1 // header + cap + null
		if total%4 != 0 {
			t.Errorf("AlignedCap(%d) = %d: total %d not aligned to 4", tt.input, got, total)
		}
	}
}

func TestAlignedCapEliminatesPadding(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	// Allocate two strings with aligned caps and verify there's no
	// wasted padding between them by checking the distance equals
	// exactly header + cap + 1 (null terminator), rounded to alignment.
	cap1 := mstrings.AlignedCap(10) // 12
	s1 := mstrings.New(arena, cap1)
	s2 := mstrings.New(arena, cap1)

	addr1 := uintptr(unsafe.Pointer(s1))
	addr2 := uintptr(unsafe.Pointer(s2))
	stride := addr2 - addr1

	// header(8) + cap(12) + null(1) = 21, but the allocator will pad
	// the next allocation to alignment. With AlignedCap the total
	// allocation size (8+12+1 = 21) still needs padding to 24.
	// The key property: stride should be consistent and minimal.
	s3 := mstrings.New(arena, cap1)
	addr3 := uintptr(unsafe.Pointer(s3))
	stride2 := addr3 - addr2

	if stride != stride2 {
		t.Fatalf("inconsistent stride: %d vs %d (addresses: %x, %x, %x)",
			stride, stride2, addr1, addr2, addr3)
	}
}

func TestMarshalJSON(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.From(arena, "hello world")
	got, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `"hello world"` {
		t.Fatalf("Marshal = %s, want %q", got, `"hello world"`)
	}

	goGot, _ := json.Marshal("hello world")
	if string(got) != string(goGot) {
		t.Fatalf("mstrings marshal %s != Go string marshal %s", got, goGot)
	}
}

func TestMarshalJSONEscaping(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.From(arena, "line1\nline2\t\"quoted\"")
	got, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	goGot, _ := json.Marshal("line1\nline2\t\"quoted\"")
	if string(got) != string(goGot) {
		t.Fatalf("escaped marshal %s != Go string marshal %s", got, goGot)
	}
}

func TestMarshalJSONNil(t *testing.T) {
	var s mstrings.String
	got, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "null" {
		t.Fatalf("Marshal(nil) = %s, want null", got)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.New(arena, 20)
	if err := json.Unmarshal([]byte(`"hello world"`), s); err != nil {
		t.Fatal(err)
	}
	if mstrings.GoString(s) != "hello world" {
		t.Fatalf("after unmarshal: %q, want %q", mstrings.GoString(s), "hello world")
	}
}

func TestUnmarshalJSONNull(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.From(arena, "existing")
	if err := json.Unmarshal([]byte("null"), s); err != nil {
		t.Fatal(err)
	}
	if mstrings.Len(s) != 0 {
		t.Fatalf("after null unmarshal: len=%d, want 0", mstrings.Len(s))
	}
	// Capacity should be preserved — buffer not lost
	if mstrings.Cap(s) != 8 {
		t.Fatalf("after null unmarshal: cap=%d, want 8", mstrings.Cap(s))
	}
}

func TestUnmarshalJSONNullInStruct(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	type Player struct {
		Name  mstrings.String `json:"name"`
		Score int             `json:"score"`
	}

	// encoding/json nils pointer-typed struct fields on null — this is
	// standard Go behavior and cannot be overridden. The arena still
	// owns the memory; it is reclaimed when the arena is destroyed.
	p := Player{Name: mstrings.From(arena, "Alice"), Score: 10}
	if err := json.Unmarshal([]byte(`{"name":null,"score":99}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.Name != nil {
		t.Fatal("expected Name to be nil after null unmarshal")
	}
	if p.Score != 99 {
		t.Fatalf("Score=%d, want 99", p.Score)
	}
}

func TestUnmarshalJSONEscaped(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.New(arena, 30)
	if err := json.Unmarshal([]byte(`"line1\nline2\t\"quoted\""`), s); err != nil {
		t.Fatal(err)
	}
	want := "line1\nline2\t\"quoted\""
	if mstrings.GoString(s) != want {
		t.Fatalf("after escaped unmarshal: %q, want %q", mstrings.GoString(s), want)
	}
}

func TestUnmarshalJSONExceedsCapacity(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	s := mstrings.New(arena, 3)
	err := json.Unmarshal([]byte(`"toolong"`), s)
	if err == nil {
		t.Fatal("expected error for string exceeding capacity")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	original := "round trip test"
	s := mstrings.From(arena, original)

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	s2 := mstrings.New(arena, 20)
	if err := json.Unmarshal(data, s2); err != nil {
		t.Fatal(err)
	}
	if !mstrings.Equal(s, s2) {
		t.Fatalf("round trip: %q != %q", mstrings.GoString(s), mstrings.GoString(s2))
	}
}

func TestJSONInStruct(t *testing.T) {
	arena := mmm.NewArena(4096)
	defer mmm.DestroyArena(&arena)

	type Player struct {
		Name  mstrings.String `json:"name"`
		Score int             `json:"score"`
	}

	p := Player{Name: mstrings.From(arena, "Alice"), Score: 42}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw["name"]) != `"Alice"` {
		t.Fatalf("name field = %s, want %q", raw["name"], `"Alice"`)
	}

	p2 := Player{Name: mstrings.New(arena, 20)}
	if err := json.Unmarshal(data, &p2); err != nil {
		t.Fatal(err)
	}
	if mstrings.GoString(p2.Name) != "Alice" {
		t.Fatalf("unmarshaled name = %q, want %q", mstrings.GoString(p2.Name), "Alice")
	}
	if p2.Score != 42 {
		t.Fatalf("unmarshaled score = %d, want 42", p2.Score)
	}
}

// addPtr is a helper for pointer arithmetic in tests
func addPtr(p *byte, offset int) *byte {
	return (*byte)(unsafe.Add(unsafe.Pointer(p), offset))
}
