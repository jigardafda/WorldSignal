// Package jsonx provides JSON marshaling that matches JavaScript's JSON.stringify
// (used by graphql-yoga and Fastify): no HTML escaping of &, <, >. Byte-parity
// with the TypeScript backend depends on this.
package jsonx

import (
	"bytes"
	"encoding/json"
)

// Unmarshal is a thin alias for json.Unmarshal, kept here so callers depend only
// on this package for JSON handling.
func Unmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// Marshal is like json.Marshal but does not HTML-escape &, <, > and emits no
// trailing newline.
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encoder.Encode appends a newline; strip it to match json.Marshal output.
	b := buf.Bytes()
	if n := len(b); n > 0 && b[n-1] == '\n' {
		b = b[:n-1]
	}
	return b, nil
}
