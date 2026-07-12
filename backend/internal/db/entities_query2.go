package db

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- Deliveries ----

// DeliveryRow is a list-view delivery joined with subscription + signal title.
type DeliveryRow struct {
	ID               string
	SubscriptionID   string
	SubscriptionName string
	Channel          string
	SignalID         string
	SignalTitle      string
	Status           string
	Attempts         int
	CreatedAt        time.Time
	DeliveredAt      *time.Time
	FailedAt         *time.Time
	ErrorMessage     *string
}

// DeliveryDetail adds the payload.
type DeliveryDetail struct {
	DeliveryRow
	Payload RawJSON
}

// DeliveryFilter filters the delivery list.
type DeliveryFilter struct {
	Status         *string
	SubscriptionID *string
	Limit          int
	Offset         int
}

// ListDeliveriesFiltered returns deliveries (filtered/paged) plus total.
func (d *DB) ListDeliveriesFiltered(ctx context.Context, f DeliveryFilter) ([]DeliveryRow, int, error) {
	var conds []string
	var args []any
	if f.Status != nil {
		args = append(args, *f.Status)
		conds = append(conds, `de."status"=$`+itoa(len(args))+`::"DeliveryStatus"`)
	}
	if f.SubscriptionID != nil {
		args = append(args, *f.SubscriptionID)
		conds = append(conds, `de."subscriptionId"=$`+itoa(len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "DeliveryEvent" de`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, clampLimit(f.Limit))
	limitP := itoa(len(args))
	args = append(args, f.Offset)
	offsetP := itoa(len(args))
	rows, err := d.Pool.Query(ctx,
		`SELECT de."id",de."subscriptionId",sub."name",de."channel",de."signalId",sig."title",
		 de."status",de."attempts",de."createdAt",de."deliveredAt",de."failedAt",de."errorMessage"
		 FROM "DeliveryEvent" de JOIN "Subscription" sub ON sub."id"=de."subscriptionId"
		 JOIN "Signal" sig ON sig."id"=de."signalId"`+where+
			` ORDER BY de."createdAt" DESC LIMIT $`+limitP+` OFFSET $`+offsetP, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []DeliveryRow{}
	for rows.Next() {
		var r DeliveryRow
		if err := rows.Scan(&r.ID, &r.SubscriptionID, &r.SubscriptionName, &r.Channel, &r.SignalID, &r.SignalTitle,
			&r.Status, &r.Attempts, &r.CreatedAt, &r.DeliveredAt, &r.FailedAt, &r.ErrorMessage); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// GetDeliveryDetail returns full delivery detail or nil.
func (d *DB) GetDeliveryDetail(ctx context.Context, id string) (*DeliveryDetail, error) {
	var r DeliveryDetail
	var payload []byte
	err := d.Pool.QueryRow(ctx,
		`SELECT de."id",de."subscriptionId",sub."name",de."channel",de."signalId",sig."title",
		 de."status",de."attempts",de."createdAt",de."deliveredAt",de."failedAt",de."errorMessage",de."payload"
		 FROM "DeliveryEvent" de JOIN "Subscription" sub ON sub."id"=de."subscriptionId"
		 JOIN "Signal" sig ON sig."id"=de."signalId" WHERE de."id"=$1`, id).
		Scan(&r.ID, &r.SubscriptionID, &r.SubscriptionName, &r.Channel, &r.SignalID, &r.SignalTitle,
			&r.Status, &r.Attempts, &r.CreatedAt, &r.DeliveredAt, &r.FailedAt, &r.ErrorMessage, &payload)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Payload = RawJSON(payload)
	return &r, nil
}

// ResetDeliveryForRetry sets a delivery back to PENDING; returns false if absent.
func (d *DB) ResetDeliveryForRetry(ctx context.Context, id string) (bool, error) {
	tag, err := d.Pool.Exec(ctx,
		`UPDATE "DeliveryEvent" SET "status"='PENDING',"errorMessage"=NULL,"failedAt"=NULL WHERE "id"=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ---- Jobs (ws_jobs queue) ----

// JobRow mirrors a ws_jobs row for the admin view.
type JobRow struct {
	ID          string
	Queue       string
	State       string
	RetryCount  int
	RetryLimit  int
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	LastError   *string
}

// JobFilter filters the jobs list.
type JobFilter struct {
	Queue  *string
	State  *string
	Limit  int
	Offset int
}

// ListJobs returns queue jobs (filtered/paged) plus total.
func (d *DB) ListJobs(ctx context.Context, f JobFilter) ([]JobRow, int, error) {
	var conds []string
	var args []any
	if f.Queue != nil {
		args = append(args, *f.Queue)
		conds = append(conds, `queue=$`+itoa(len(args)))
	}
	if f.State != nil {
		args = append(args, *f.State)
		conds = append(conds, `state=$`+itoa(len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM ws_jobs`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, clampLimit(f.Limit))
	limitP := itoa(len(args))
	args = append(args, f.Offset)
	offsetP := itoa(len(args))
	rows, err := d.Pool.Query(ctx,
		`SELECT id,queue,state,retry_count,retry_limit,created_at,started_at,completed_at,last_error
		 FROM ws_jobs`+where+` ORDER BY created_at DESC LIMIT $`+limitP+` OFFSET $`+offsetP, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []JobRow{}
	for rows.Next() {
		var j JobRow
		if err := rows.Scan(&j.ID, &j.Queue, &j.State, &j.RetryCount, &j.RetryLimit, &j.CreatedAt, &j.StartedAt, &j.CompletedAt, &j.LastError); err != nil {
			return nil, 0, err
		}
		out = append(out, j)
	}
	return out, total, rows.Err()
}

// JobStateCounts returns job counts grouped by state.
func (d *DB) JobStateCounts(ctx context.Context) ([]Bucket, error) {
	return d.bucketQuery(ctx, `SELECT state, count(*) FROM ws_jobs GROUP BY state ORDER BY state`)
}

// RetryJob requeues a failed job (state→created, run now); returns false if absent.
func (d *DB) RetryJob(ctx context.Context, id string) (bool, error) {
	tag, err := d.Pool.Exec(ctx,
		`UPDATE ws_jobs SET state='created', start_after=now(), last_error=NULL WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ---- Taxonomy counts ----

// TaxonomyCounts returns signal counts per tag code.
func (d *DB) TaxonomyCounts(ctx context.Context) (map[string]int, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT tt."code", count(st."signalId") FROM "TaxonomyTag" tt
		 LEFT JOIN "SignalTag" st ON st."tagId"=tt."id" GROUP BY tt."code"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return nil, err
		}
		out[code] = n
	}
	return out, rows.Err()
}

// ---- Source update/delete ----

// SourceFullPatch holds editable source fields.
type SourceFullPatch struct {
	Name           *string
	Country        *string
	Priority       *int
	Credibility    *float64
	CrawlFrequency *int
	Enabled        *bool
}

// UpdateSourceFull applies a partial source update and returns the row.
func (d *DB) UpdateSourceFull(ctx context.Context, id string, p SourceFullPatch) (*Source, error) {
	sets := `"updatedAt"=now()`
	args := []any{id}
	add := func(col string, v any) {
		args = append(args, v)
		sets += `, "` + col + `"=$` + itoa(len(args))
	}
	if p.Name != nil {
		add("name", *p.Name)
	}
	if p.Country != nil {
		add("country", *p.Country)
	}
	if p.Priority != nil {
		add("priority", *p.Priority)
	}
	if p.Credibility != nil {
		add("credibility", *p.Credibility)
	}
	if p.CrawlFrequency != nil {
		add("crawlFrequency", *p.CrawlFrequency)
	}
	if p.Enabled != nil {
		add("enabled", *p.Enabled)
	}
	row := d.Pool.QueryRow(ctx, `UPDATE "Source" SET `+sets+` WHERE "id"=$1 RETURNING `+sourceColumns, args...)
	s, err := scanSource(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// DeleteSource removes a source; returns false if absent.
func (d *DB) DeleteSource(ctx context.Context, id string) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM "Source" WHERE "id"=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ---- Subscription update/delete ----

// SubscriptionPatch holds editable subscription fields.
type SubscriptionPatch struct {
	Name    *string
	Enabled *bool
	Filter  RawJSON
	Config  RawJSON
}

// UpdateSubscription applies a partial update and returns the scalar row.
func (d *DB) UpdateSubscription(ctx context.Context, id string, p SubscriptionPatch) (*Subscription, error) {
	sets := []string{}
	args := []any{id}
	if p.Name != nil {
		args = append(args, *p.Name)
		sets = append(sets, `"name"=$`+itoa(len(args)))
	}
	if p.Enabled != nil {
		args = append(args, *p.Enabled)
		sets = append(sets, `"enabled"=$`+itoa(len(args)))
	}
	if p.Filter != nil {
		args = append(args, []byte(p.Filter))
		sets = append(sets, `"filter"=$`+itoa(len(args)))
	}
	if p.Config != nil {
		args = append(args, []byte(p.Config))
		sets = append(sets, `"config"=$`+itoa(len(args)))
	}
	if len(sets) == 0 {
		return d.getSubscriptionBasic(ctx, id)
	}
	s, err := scanSubscription(d.Pool.QueryRow(ctx, `UPDATE "Subscription" SET `+strings.Join(sets, ", ")+` WHERE "id"=$1 RETURNING `+subscriptionCols, args...))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// DeleteSubscription removes a subscription; returns false if absent.
func (d *DB) DeleteSubscription(ctx context.Context, id string) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM "Subscription" WHERE "id"=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
