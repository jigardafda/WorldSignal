package db

// RawJSON holds raw jsonb bytes and marshals them through unchanged (null when
// nil), matching how Prisma surfaces a Json? column.
type RawJSON []byte

// MarshalJSON returns the raw bytes, or null when empty.
func (r RawJSON) MarshalJSON() ([]byte, error) {
	if len(r) == 0 {
		return []byte("null"), nil
	}
	return r, nil
}
