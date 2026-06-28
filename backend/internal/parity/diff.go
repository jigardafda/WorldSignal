package parity

import (
	"fmt"
)

// DiffBytes returns a human-readable description of the first divergence between
// two byte slices, or "" if they are identical.
func DiffBytes(a, b []byte) string {
	if string(a) == string(b) {
		return ""
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	idx := -1
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			idx = i
			break
		}
	}
	if idx == -1 {
		idx = n // one is a prefix of the other
	}
	return fmt.Sprintf("byte mismatch at offset %d (len a=%d b=%d)\n  a: %s\n  b: %s",
		idx, len(a), len(b), window(a, idx), window(b, idx))
}

func window(b []byte, idx int) string {
	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + 40
	if end > len(b) {
		end = len(b)
	}
	return fmt.Sprintf("…%q…", string(b[start:end]))
}
