package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/worldsignal/backend/internal/cuid"
)

// RawItem holds the fields of a RawItem needed by normalization.
type RawItem struct {
	ID          string
	SourceID    string
	RawURL      *string
	SourceGuid  *string
	RawTitle    *string
	RawContent  *string
	ContentHash *string
	PublishedAt *time.Time
	Status      string
}

// GetRawItem loads a raw item by id (nil if not found).
func (d *DB) GetRawItem(ctx context.Context, id string) (*RawItem, error) {
	var r RawItem
	err := d.Pool.QueryRow(ctx,
		`SELECT "id","sourceId","rawUrl","sourceGuid","rawTitle","rawContent","contentHash","publishedAt","status" FROM "RawItem" WHERE "id"=$1`, id).
		Scan(&r.ID, &r.SourceID, &r.RawURL, &r.SourceGuid, &r.RawTitle, &r.RawContent, &r.ContentHash, &r.PublishedAt, &r.Status)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// SetRawItemStatus updates a raw item's status.
func (d *DB) SetRawItemStatus(ctx context.Context, id, status string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE "RawItem" SET "status"=$2::"RawItemStatus" WHERE "id"=$1`, id, status)
	return err
}

// ArticleIDByRawItem returns the article id linked to a raw item, or "".
func (d *DB) ArticleIDByRawItem(ctx context.Context, rawItemID string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `SELECT "id" FROM "Article" WHERE "rawItemId"=$1`, rawItemID).Scan(&id)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return id, err
}

// FindDuplicateArticle returns the id of an article with the same content hash or
// canonical URL (exact dedupe), or "".
func (d *DB) FindDuplicateArticle(ctx context.Context, contentHash string, canonicalURL *string) (string, error) {
	var id string
	var err error
	if canonicalURL != nil {
		err = d.Pool.QueryRow(ctx,
			`SELECT "id" FROM "Article" WHERE "contentHash"=$1 OR "canonicalUrl"=$2 LIMIT 1`, contentHash, *canonicalURL).Scan(&id)
	} else {
		err = d.Pool.QueryRow(ctx,
			`SELECT "id" FROM "Article" WHERE "contentHash"=$1 LIMIT 1`, contentHash).Scan(&id)
	}
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return id, err
}

// NewArticle is the data for creating an Article.
type NewArticle struct {
	RawItemID    string
	SourceID     string
	CanonicalURL *string
	Title        string
	Body         *string
	Summary      *string
	PublishedAt  *time.Time
	ContentHash  string
	TokenSet     string
}

// CreateArticle inserts an Article and returns its id.
func (d *DB) CreateArticle(ctx context.Context, a NewArticle) (string, error) {
	id := cuid.New()
	_, err := d.Pool.Exec(ctx,
		`INSERT INTO "Article" ("id","rawItemId","sourceId","canonicalUrl","title","body","summary","publishedAt","contentHash","tokenSet")
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		id, a.RawItemID, a.SourceID, a.CanonicalURL, a.Title, a.Body, a.Summary, a.PublishedAt, a.ContentHash, a.TokenSet)
	return id, err
}
