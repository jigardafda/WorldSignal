package db

import (
	"context"
	"strings"
)

// EntityRow is a distinct extracted entity with how many signals mention it.
type EntityRow struct {
	Name        string
	Type        string
	SignalCount int
}

// EntityFilter narrows the entity listing.
type EntityFilter struct {
	Search *string // substring match on the entity name
	Type   *string // entityType code (PERSON/ORG/GOVERNMENT/…)
	Limit  int
}

// SearchEntities returns distinct entities extracted across signals — the name,
// its type, and the number of signals that mention it — most-mentioned first.
// Entities are stored as SignalAttribute rows (key='entity', valueCode=type,
// valueText=name); this makes them first-class queryable.
func (d *DB) SearchEntities(ctx context.Context, f EntityFilter) ([]EntityRow, error) {
	conds := []string{`sa."key"='entity'`, `sa."valueText" <> ''`}
	var args []any
	if f.Search != nil && strings.TrimSpace(*f.Search) != "" {
		args = append(args, "%"+strings.TrimSpace(*f.Search)+"%")
		conds = append(conds, `sa."valueText" ILIKE $`+itoa(len(args)))
	}
	if f.Type != nil && *f.Type != "" {
		args = append(args, *f.Type)
		conds = append(conds, `sa."valueCode" = $`+itoa(len(args)))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	args = append(args, limit)
	limitP := itoa(len(args))

	q := `SELECT sa."valueText", sa."valueCode", count(DISTINCT sa."signalId") AS c
		FROM "SignalAttribute" sa
		WHERE ` + strings.Join(conds, " AND ") + `
		GROUP BY sa."valueText", sa."valueCode"
		ORDER BY c DESC, sa."valueText" ASC
		LIMIT $` + limitP
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EntityRow{}
	for rows.Next() {
		var e EntityRow
		if err := rows.Scan(&e.Name, &e.Type, &e.SignalCount); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
