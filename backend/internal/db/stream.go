package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/worldsignal/backend/internal/cuid"
)

// StreamSubscription is the minimal view a streaming/polling client needs to
// resolve and read a subscription's feed.
type StreamSubscription struct {
	ID      string
	Name    string
	Channel string
	Enabled bool
}

// GetStreamSubscription looks up a subscription by id, or (nil, nil) if absent.
func (d *DB) GetStreamSubscription(ctx context.Context, id string) (*StreamSubscription, error) {
	var s StreamSubscription
	err := d.Pool.QueryRow(ctx,
		`SELECT "id","name","channel","enabled" FROM "Subscription" WHERE "id"=$1`, id).
		Scan(&s.ID, &s.Name, &s.Channel, &s.Enabled)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// SendTestDelivery creates (or refreshes) a delivery for a subscription using
// the most recent real signal, marked as a test event, so the owner can verify
// their client end to end without waiting for a live match. It references an
// existing signal (no synthetic rows) and bumps seq on conflict so streaming
// cursors treat it as new. Returns the delivery id, or "" if there are no
// signals yet.
func (d *DB) SendTestDelivery(ctx context.Context, subID string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
WITH sig AS (SELECT * FROM "Signal" ORDER BY "lastSeenAt" DESC LIMIT 1)
INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","createdAt")
SELECT $2::text, $1::text, sig.id, (SELECT "channel" FROM "Subscription" WHERE "id"=$1), 'SENT',
  jsonb_build_object(
    'schema_version','2026-06-01', 'event_type','signal.test', 'event_id',$2::text,
    'created_at', to_char(now() AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
    'subscription_id',$1::text, 'test',true,
    'data', jsonb_build_object(
      'signal_id',sig.id,'title','[TEST] '||sig."title",'summary',sig."summary",
      'status',sig."status",'severity',sig."severity",'confidence',sig."confidence",
      'country',sig."country",'source_count',sig."sourceCount")
  ), now()
FROM sig
ON CONFLICT ("subscriptionId","signalId") DO UPDATE
  SET "seq"=nextval('"DeliveryEvent_seq_seq"'), "payload"=EXCLUDED."payload",
      "createdAt"=now(), "status"='SENT'
RETURNING "id"`, subID, cuid.New()).Scan(&id)
	if err == pgx.ErrNoRows {
		return "", nil // no signals to test with yet
	}
	return id, err
}

// MaxDeliverySeq returns the highest delivery seq for a subscription (0 if none)
// — the cursor a live-tail stream starts from so it emits only new events.
func (d *DB) MaxDeliverySeq(ctx context.Context, subID string) (int64, error) {
	var seq int64
	err := d.Pool.QueryRow(ctx,
		`SELECT COALESCE(MAX("seq"),0) FROM "DeliveryEvent" WHERE "subscriptionId"=$1`, subID).Scan(&seq)
	return seq, err
}

// StreamDelivery is one row of a subscription's durable delivery feed.
type StreamDelivery struct {
	Seq       int64
	ID        string
	SignalID  string
	Channel   string
	Payload   RawJSON
	CreatedAt time.Time
}

// ListDeliveriesForStream returns up to limit delivery rows for a subscription
// with seq > sinceSeq, oldest first. This keyset feed backs polling and the
// SSE/WebSocket transports; seq is monotonic so a client resumes exactly where
// it left off with no gaps or duplicates.
func (d *DB) ListDeliveriesForStream(ctx context.Context, subID string, sinceSeq int64, limit int) ([]StreamDelivery, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := d.Pool.Query(ctx,
		`SELECT "seq","id","signalId","channel","payload","createdAt" FROM "DeliveryEvent"
		 WHERE "subscriptionId"=$1 AND "seq">$2 ORDER BY "seq" ASC LIMIT $3`, subID, sinceSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StreamDelivery
	for rows.Next() {
		var e StreamDelivery
		var payload []byte
		if err := rows.Scan(&e.Seq, &e.ID, &e.SignalID, &e.Channel, &payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Payload = RawJSON(payload)
		out = append(out, e)
	}
	return out, rows.Err()
}
