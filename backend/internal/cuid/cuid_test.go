package cuid

import (
	"regexp"
	"testing"
)

var shape = regexp.MustCompile(`^c[0-9a-z]+$`)

func TestNewShape(t *testing.T) {
	id := New()
	if !shape.MatchString(id) {
		t.Fatalf("id %q not cuid-shaped", id)
	}
	if id[0] != 'c' {
		t.Fatalf("id must start with c: %q", id)
	}
	if len(id) < 20 {
		t.Fatalf("id too short: %q", id)
	}
}

func TestNewUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 10000; i++ {
		id := New()
		if seen[id] {
			t.Fatalf("duplicate id generated: %q", id)
		}
		seen[id] = true
	}
}

func TestToBase36(t *testing.T) {
	cases := map[uint64]string{0: "0", 1: "1", 35: "z", 36: "10"}
	for in, want := range cases {
		if got := toBase36(in); got != want {
			t.Fatalf("toBase36(%d)=%q want %q", in, got, want)
		}
	}
}

func TestPad(t *testing.T) {
	if got := pad("a", 4); got != "000a" {
		t.Fatalf("pad short got %q", got)
	}
	if got := pad("abcdef", 4); got != "cdef" {
		t.Fatalf("pad long should keep last 4, got %q", got)
	}
}
