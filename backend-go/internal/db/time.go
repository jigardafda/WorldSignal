package db

import "time"

// isoLayout matches JavaScript Date.prototype.toISOString(): UTC, millisecond
// precision, trailing Z. Prisma serializes DateTime this way, so byte-parity of
// any timestamp field depends on it.
const isoLayout = "2006-01-02T15:04:05.000Z"

// PrismaTime wraps time.Time to marshal like a Prisma DateTime.
type PrismaTime struct{ time.Time }

// MarshalJSON renders the time as a JS ISO-8601 string.
func (t PrismaTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.UTC().Format(isoLayout) + `"`), nil
}

// NewTime wraps a time.Time.
func NewTime(t time.Time) PrismaTime { return PrismaTime{t} }

// NewTimePtr wraps a *time.Time, preserving nil.
func NewTimePtr(t *time.Time) *PrismaTime {
	if t == nil {
		return nil
	}
	return &PrismaTime{*t}
}
