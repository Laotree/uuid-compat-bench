package uuid

import (
	"testing"

	guuid "github.com/google/uuid"
	"uuid"
)

func TestProvidersGenerate(t *testing.T) {
	for _, p := range All {
		a := p.New()
		b := p.New()
		if a == b {
			t.Fatalf("%s: generated duplicate UUID", p.Name())
		}
		if p.String(a) != guuid.UUID(a).String() {
			t.Fatalf("%s: string encoding mismatch", p.Name())
		}
	}
}

func TestProvidersParseRoundtrip(t *testing.T) {
	for _, p := range All {
		for _, s := range []string{
			"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			"00000000-0000-0000-0000-000000000000",
		} {
			b, err := p.Parse(s)
			if err != nil {
				t.Fatalf("%s: parse %q failed: %v", p.Name(), s, err)
			}
			if p.String(b) != s {
				t.Fatalf("%s: parse/string mismatch: %q != %q", p.Name(), p.String(b), s)
			}
		}
	}
}

func TestProvidersParseInvalid(t *testing.T) {
	for _, p := range All {
		if _, err := p.Parse("not-a-uuid"); err == nil {
			t.Fatalf("%s: expected error for invalid input", p.Name())
		}
	}
}

func TestCrossParseCompatibility(t *testing.T) {
	g, s := Google{}, Stdlib{}
	b := g.New()
	parsed, err := s.Parse(g.String(b))
	if err != nil {
		t.Fatalf("stdlib parse of google string failed: %v", err)
	}
	if parsed != b {
		t.Fatal("cross-parse mismatch")
	}
}

func TestInsertValuePreservesBytes(t *testing.T) {
	google, stdlib := Google{}, Stdlib{}
	b := google.New()
	if got := google.InsertValue(b).(guuid.UUID); got != guuid.UUID(b) {
		t.Fatal("google insert value mismatch")
	}
	if got := stdlib.InsertValue(b).(uuid.UUID); got != uuid.UUID(b) {
		t.Fatal("stdlib insert value mismatch")
	}
}

func TestScanValue(t *testing.T) {
	google, stdlib := Google{}, Stdlib{}
	sample := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	for _, p := range []Provider{google, stdlib} {
		var b [16]byte
		v := p.ScanValue(&b)
		switch tv := v.(type) {
		case *guuid.UUID:
			*tv = guuid.UUID(sample)
		case *uuid.UUID:
			*tv = uuid.UUID(sample)
		default:
			t.Fatalf("%s: unexpected scan value type %T", p.Name(), v)
		}
		if b != sample {
			t.Fatalf("%s: scan value did not write into backing array", p.Name())
		}
	}
}
