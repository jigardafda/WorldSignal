package db

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- Articles ----

// ArticleRow is a list-view article joined with its source + signal count.
type ArticleRow struct {
	ID           string
	Title        string
	CanonicalURL *string
	Summary      *string
	PublishedAt  *time.Time
	FetchedAt    time.Time
	SourceID     string
	SourceName   string
	SignalCount  int
}

// ArticleDetail adds body/metadata and linked signals.
type ArticleDetail struct {
	ArticleRow
	Body        *string
	Author      *string
	Language    *string
	Country     *string
	ContentHash *string
	TokenSet    *string
	Signals     []LinkedSignal
}

// LinkedSignal is a signal an article belongs to.
type LinkedSignal struct {
	ID           string
	Title        string
	RelationType string
	Similarity   *float64
}

// ListFilter is a generic list filter.
type ListFilter struct {
	SourceID *string
	Status   *string
	Search   *string
	Limit    int
	Offset   int
}

func clampLimit(l int) int {
	if l <= 0 {
		return 50
	}
	if l > 200 {
		return 200
	}
	return l
}

// ListArticles returns articles (filtered/paged) plus the total count.
func (d *DB) ListArticles(ctx context.Context, f ListFilter) ([]ArticleRow, int, error) {
	var conds []string
	var args []any
	if f.SourceID != nil {
		args = append(args, *f.SourceID)
		conds = append(conds, `a."sourceId"=$`+itoa(len(args)))
	}
	if f.Search != nil {
		args = append(args, "%"+*f.Search+"%")
		conds = append(conds, `a."title" ILIKE $`+itoa(len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "Article" a`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, clampLimit(f.Limit))
	limitP := itoa(len(args))
	args = append(args, f.Offset)
	offsetP := itoa(len(args))
	q := `SELECT a."id",a."title",a."canonicalUrl",a."summary",a."publishedAt",a."fetchedAt",
		a."sourceId",s."name",(SELECT count(*) FROM "SignalArticle" sa WHERE sa."articleId"=a."id")
		FROM "Article" a JOIN "Source" s ON s."id"=a."sourceId"` + where +
		` ORDER BY a."fetchedAt" DESC LIMIT $` + limitP + ` OFFSET $` + offsetP
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []ArticleRow{}
	for rows.Next() {
		var a ArticleRow
		if err := rows.Scan(&a.ID, &a.Title, &a.CanonicalURL, &a.Summary, &a.PublishedAt, &a.FetchedAt, &a.SourceID, &a.SourceName, &a.SignalCount); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// GetArticle returns full article detail with linked signals, or nil.
func (d *DB) GetArticle(ctx context.Context, id string) (*ArticleDetail, error) {
	var a ArticleDetail
	err := d.Pool.QueryRow(ctx,
		`SELECT a."id",a."title",a."canonicalUrl",a."summary",a."publishedAt",a."fetchedAt",a."sourceId",s."name",
		 a."body",a."author",a."language",a."country",a."contentHash",a."tokenSet"
		 FROM "Article" a JOIN "Source" s ON s."id"=a."sourceId" WHERE a."id"=$1`, id).
		Scan(&a.ID, &a.Title, &a.CanonicalURL, &a.Summary, &a.PublishedAt, &a.FetchedAt, &a.SourceID, &a.SourceName,
			&a.Body, &a.Author, &a.Language, &a.Country, &a.ContentHash, &a.TokenSet)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := d.Pool.Query(ctx,
		`SELECT s."id",s."title",sa."relationType",sa."similarityScore"
		 FROM "SignalArticle" sa JOIN "Signal" s ON s."id"=sa."signalId"
		 WHERE sa."articleId"=$1 ORDER BY sa."addedAt" ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ls LinkedSignal
		if err := rows.Scan(&ls.ID, &ls.Title, &ls.RelationType, &ls.Similarity); err != nil {
			return nil, err
		}
		a.Signals = append(a.Signals, ls)
	}
	return &a, rows.Err()
}

// ---- Raw items ----

// RawItemRow is a list-view raw item.
type RawItemRow struct {
	ID          string
	SourceID    string
	SourceName  string
	SourceGuid  *string
	RawURL      *string
	RawTitle    *string
	Status      string
	PublishedAt *time.Time
	FetchedAt   time.Time
}

// RawItemDetail adds raw content + payload.
type RawItemDetail struct {
	RawItemRow
	RawContent  *string
	ContentHash *string
	RawPayload  RawJSON
}

// ListRawItems returns raw items (filtered/paged) plus total count.
func (d *DB) ListRawItems(ctx context.Context, f ListFilter) ([]RawItemRow, int, error) {
	var conds []string
	var args []any
	if f.SourceID != nil {
		args = append(args, *f.SourceID)
		conds = append(conds, `r."sourceId"=$`+itoa(len(args)))
	}
	if f.Status != nil {
		args = append(args, *f.Status)
		conds = append(conds, `r."status"=$`+itoa(len(args))+`::"RawItemStatus"`)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "RawItem" r`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, clampLimit(f.Limit))
	limitP := itoa(len(args))
	args = append(args, f.Offset)
	offsetP := itoa(len(args))
	rows, err := d.Pool.Query(ctx,
		`SELECT r."id",r."sourceId",s."name",r."sourceGuid",r."rawUrl",r."rawTitle",r."status",r."publishedAt",r."fetchedAt"
		 FROM "RawItem" r JOIN "Source" s ON s."id"=r."sourceId"`+where+
			` ORDER BY r."fetchedAt" DESC LIMIT $`+limitP+` OFFSET $`+offsetP, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []RawItemRow{}
	for rows.Next() {
		var r RawItemRow
		if err := rows.Scan(&r.ID, &r.SourceID, &r.SourceName, &r.SourceGuid, &r.RawURL, &r.RawTitle, &r.Status, &r.PublishedAt, &r.FetchedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// GetRawItemDetail returns full raw item detail or nil.
func (d *DB) GetRawItemDetail(ctx context.Context, id string) (*RawItemDetail, error) {
	var r RawItemDetail
	var payload []byte
	err := d.Pool.QueryRow(ctx,
		`SELECT r."id",r."sourceId",s."name",r."sourceGuid",r."rawUrl",r."rawTitle",r."status",r."publishedAt",r."fetchedAt",
		 r."rawContent",r."contentHash",r."rawPayload"
		 FROM "RawItem" r JOIN "Source" s ON s."id"=r."sourceId" WHERE r."id"=$1`, id).
		Scan(&r.ID, &r.SourceID, &r.SourceName, &r.SourceGuid, &r.RawURL, &r.RawTitle, &r.Status, &r.PublishedAt, &r.FetchedAt,
			&r.RawContent, &r.ContentHash, &payload)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.RawPayload = RawJSON(payload)
	return &r, nil
}
