package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/jsonx"
)

// ClusterArticle holds the article fields used by clustering.
type ClusterArticle struct {
	ID          string
	Title       string
	Summary     *string
	Country     *string
	TokenSet    string
	PublishedAt *time.Time
}

// GetClusterArticle loads the fields needed to cluster an article (nil if absent).
func (d *DB) GetClusterArticle(ctx context.Context, id string) (*ClusterArticle, error) {
	var a ClusterArticle
	var tokenSet *string
	err := d.Pool.QueryRow(ctx,
		`SELECT "id","title","summary","country","tokenSet","publishedAt" FROM "Article" WHERE "id"=$1`, id).
		Scan(&a.ID, &a.Title, &a.Summary, &a.Country, &tokenSet, &a.PublishedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.TokenSet = deref(tokenSet)
	return &a, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ExistingSignalForArticle returns the signal id already linked to an article, or "".
func (d *DB) ExistingSignalForArticle(ctx context.Context, articleID string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `SELECT "signalId" FROM "SignalArticle" WHERE "articleId"=$1 LIMIT 1`, articleID).Scan(&id)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return id, err
}

// RecentSignalCandidate is a clustering candidate.
type RecentSignalCandidate struct {
	ID       string
	TokenSet string
}

// RecentSignalCandidates returns up to 300 signals seen since `since`, newest first.
func (d *DB) RecentSignalCandidates(ctx context.Context, since time.Time) ([]RecentSignalCandidate, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT "id", COALESCE("metadata"->>'tokenSet','') FROM "Signal" WHERE "lastSeenAt" >= $1 ORDER BY "lastSeenAt" DESC LIMIT 300`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecentSignalCandidate
	for rows.Next() {
		var c RecentSignalCandidate
		if err := rows.Scan(&c.ID, &c.TokenSet); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AttachArticleToSignal links an article as SUPPORTING and bumps the signal.
func (d *DB) AttachArticleToSignal(ctx context.Context, signalID, articleID string, score float64, now time.Time) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore") VALUES ($1,$2,'SUPPORTING',$3)`,
		signalID, articleID, score); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE "Signal" SET "sourceCount"="sourceCount"+1, "lastSeenAt"=$2 WHERE "id"=$1`, signalID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CreateSignalFromArticle creates a new Signal with the article as PRIMARY.
func (d *DB) CreateSignalFromArticle(ctx context.Context, a *ClusterArticle, now time.Time) (string, error) {
	summary := a.Title
	if a.Summary != nil && *a.Summary != "" {
		summary = *a.Summary
	}
	firstSeen := now
	if a.PublishedAt != nil {
		firstSeen = *a.PublishedAt
	}
	meta, _ := jsonx.Marshal(struct {
		TokenSet string `json:"tokenSet"`
	}{a.TokenSet})

	id := cuid.New()
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO "Signal" ("id","title","summary","status","firstSeenAt","lastSeenAt","country","sourceCount","metadata","updatedAt")
		 VALUES ($1,$2,$3,'UNVERIFIED',$4,$5,$6,1,$7,now())`,
		id, a.Title, summary, firstSeen, now, a.Country, meta); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO "SignalArticle" ("signalId","articleId","relationType","similarityScore") VALUES ($1,$2,'PRIMARY',1)`,
		id, a.ID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}
