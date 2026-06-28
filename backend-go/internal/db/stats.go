package db

import "context"

// Stats holds aggregate counts for the stats endpoints.
type Stats struct {
	Sources           int
	Articles          int
	Signals           int
	DeliveriesSent    int
	DeliveriesPending int
}

// GetStats computes the counts used by /v1/stats and the GraphQL stats query.
func (d *DB) GetStats(ctx context.Context) (Stats, error) {
	var s Stats
	q := `SELECT
		(SELECT count(*) FROM "Source"),
		(SELECT count(*) FROM "Article"),
		(SELECT count(*) FROM "Signal"),
		(SELECT count(*) FROM "DeliveryEvent" WHERE "status"='SENT'),
		(SELECT count(*) FROM "DeliveryEvent" WHERE "status" IN ('PENDING','RETRYING'))`
	err := d.Pool.QueryRow(ctx, q).Scan(&s.Sources, &s.Articles, &s.Signals, &s.DeliveriesSent, &s.DeliveriesPending)
	return s, err
}
