package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

func scanSource(row pgx.Row) (*Source, error) {
	var s Source
	var (
		config                                []byte
		lastFetched, lastSuccess, lastFailure *time.Time
		createdAt, updatedAt                  time.Time
	)
	err := row.Scan(
		&s.ID, &s.Name, &s.Type, &s.URL, &s.Country, &s.Region, &s.Language, &s.Category,
		&s.Priority, &s.Credibility, &s.CrawlFrequency, &s.ParserType, &s.Enabled, &config,
		&lastFetched, &lastSuccess, &lastFailure, &s.FailureCount, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.Config = RawJSON(config)
	s.LastFetchedAt = NewTimePtr(lastFetched)
	s.LastSuccessAt = NewTimePtr(lastSuccess)
	s.LastFailureAt = NewTimePtr(lastFailure)
	s.CreatedAt = NewTime(createdAt)
	s.UpdatedAt = NewTime(updatedAt)
	return &s, nil
}

// ListSources returns all sources ordered by priority asc then name asc,
// mirroring prisma.source.findMany({ orderBy: [{priority:'asc'},{name:'asc'}] }).
func (d *DB) ListSources(ctx context.Context) ([]*Source, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+sourceColumns+` FROM "Source" ORDER BY "priority" ASC, "name" ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Source{}
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetSource returns a single source by id, or (nil, nil) if not found.
func (d *DB) GetSource(ctx context.Context, id string) (*Source, error) {
	row := d.Pool.QueryRow(ctx, `SELECT `+sourceColumns+` FROM "Source" WHERE "id"=$1`, id)
	s, err := scanSource(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return s, err
}
