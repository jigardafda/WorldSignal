package db

import "context"

// Bucket is a labeled count used by analytics group-by queries.
type Bucket struct {
	Key   string
	Count int
}

func (d *DB) bucketQuery(ctx context.Context, sql string, args ...any) ([]Bucket, error) {
	rows, err := d.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Bucket{}
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.Key, &b.Count); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// SignalsBySeverity counts signals grouped by severity.
func (d *DB) SignalsBySeverity(ctx context.Context) ([]Bucket, error) {
	return d.bucketQuery(ctx, `SELECT "severity"::text, count(*) FROM "Signal" GROUP BY "severity" ORDER BY count(*) DESC`)
}

// SignalsByStatus counts signals grouped by status.
func (d *DB) SignalsByStatus(ctx context.Context) ([]Bucket, error) {
	return d.bucketQuery(ctx, `SELECT "status"::text, count(*) FROM "Signal" GROUP BY "status" ORDER BY count(*) DESC`)
}

// SignalsByEventType returns the top event types by signal count.
func (d *DB) SignalsByEventType(ctx context.Context, limit int) ([]Bucket, error) {
	return d.bucketQuery(ctx,
		`SELECT COALESCE("eventType",'(none)'), count(*) FROM "Signal" GROUP BY "eventType" ORDER BY count(*) DESC LIMIT $1`, limit)
}

// SignalsByCountry returns the top countries by signal count.
func (d *DB) SignalsByCountry(ctx context.Context, limit int) ([]Bucket, error) {
	return d.bucketQuery(ctx,
		`SELECT COALESCE("country",'(unknown)'), count(*) FROM "Signal" GROUP BY "country" ORDER BY count(*) DESC LIMIT $1`, limit)
}

// SignalsOverTime returns daily signal counts for the last n days (oldest first).
func (d *DB) SignalsOverTime(ctx context.Context, days int) ([]Bucket, error) {
	return d.bucketQuery(ctx,
		`SELECT to_char(date_trunc('day', "createdAt"),'YYYY-MM-DD'), count(*)
		 FROM "Signal" WHERE "createdAt" >= now() - ($1 || ' days')::interval
		 GROUP BY 1 ORDER BY 1 ASC`, itoa(days))
}

// TopSource pairs a source with its article count.
type TopSource struct {
	ID           string
	Name         string
	ArticleCount int
}

// TopSources returns the sources producing the most articles.
func (d *DB) TopSources(ctx context.Context, limit int) ([]TopSource, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT s."id", s."name", count(a."id")
		 FROM "Source" s LEFT JOIN "Article" a ON a."sourceId"=s."id"
		 GROUP BY s."id" ORDER BY count(a."id") DESC, s."name" ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TopSource{}
	for rows.Next() {
		var t TopSource
		if err := rows.Scan(&t.ID, &t.Name, &t.ArticleCount); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeliveryStats holds delivery counts by status.
type DeliveryStats struct {
	Total        int
	Sent         int
	Pending      int
	Retrying     int
	Failed       int
	DeadLettered int
}

// GetDeliveryStats computes delivery counts by status.
func (d *DB) GetDeliveryStats(ctx context.Context) (DeliveryStats, error) {
	var s DeliveryStats
	err := d.Pool.QueryRow(ctx, `SELECT
		count(*),
		count(*) FILTER (WHERE "status"='SENT'),
		count(*) FILTER (WHERE "status"='PENDING'),
		count(*) FILTER (WHERE "status"='RETRYING'),
		count(*) FILTER (WHERE "status"='FAILED'),
		count(*) FILTER (WHERE "status"='DEAD_LETTERED')
		FROM "DeliveryEvent"`).
		Scan(&s.Total, &s.Sent, &s.Pending, &s.Retrying, &s.Failed, &s.DeadLettered)
	return s, err
}

// IngestionStats holds raw-item / article ingestion counts.
type IngestionStats struct {
	RawItems   int
	Parsed     int
	Duplicates int
	Failed     int
	Articles   int
}

// GetIngestionStats computes ingestion funnel counts.
func (d *DB) GetIngestionStats(ctx context.Context) (IngestionStats, error) {
	var s IngestionStats
	err := d.Pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM "RawItem"),
		(SELECT count(*) FROM "RawItem" WHERE "status"='PARSED'),
		(SELECT count(*) FROM "RawItem" WHERE "status"='DUPLICATE'),
		(SELECT count(*) FROM "RawItem" WHERE "status"='FAILED'),
		(SELECT count(*) FROM "Article")`).
		Scan(&s.RawItems, &s.Parsed, &s.Duplicates, &s.Failed, &s.Articles)
	return s, err
}
