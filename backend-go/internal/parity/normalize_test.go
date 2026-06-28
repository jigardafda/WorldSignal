package parity_test

import (
	"encoding/json"
	"testing"
)

// volatileKeys are non-deterministic, system-generated fields blanked before
// row/response comparison (cuid ids, timestamps).
var volatileKeys = map[string]bool{
	"id": true, "createdAt": true, "updatedAt": true,
	"lastFetchedAt": true, "lastSuccessAt": true, "lastFailureAt": true,
	"firstSeenAt": true, "lastSeenAt": true, "publishedAt": true,
	"deliveredAt": true, "failedAt": true, "addedAt": true, "fetchedAt": true,
	// Delivery envelope fields that embed non-deterministic ids/timestamps.
	"created_at": true, "signal_id": true, "event_id": true, "last_seen_at": true,
}

// normalizeJSON parses JSON, blanks volatile keys recursively, and re-marshals
// with sorted keys for a stable, comparable representation.
func normalizeJSON(t *testing.T, b []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		// Non-JSON bodies (e.g. GraphQL "true") compare as-is.
		return string(b)
	}
	return string(canonical(blank(v)))
}

func blank(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if volatileKeys[k] {
				x[k] = "<normalized>"
			} else {
				x[k] = blank(val)
			}
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = blank(val)
		}
		return x
	default:
		return v
	}
}

// canonical marshals with sorted map keys (encoding/json already sorts map keys).
func canonical(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
