package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/worldsignal/backend/internal/cuid"
)

// SignalForMatch holds the signal fields used for subscription matching and the
// delivery envelope.
type SignalForMatch struct {
	ID          string
	Title       string
	Summary     string
	Status      string
	Severity    string
	Confidence  float64
	Country     *string
	SourceCount int
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	TagCodes    []string
}

// LoadSignalForMatch loads a signal and its tag codes (nil if absent).
func (d *DB) LoadSignalForMatch(ctx context.Context, signalID string) (*SignalForMatch, error) {
	var s SignalForMatch
	err := d.Pool.QueryRow(ctx,
		`SELECT "id","title","summary","status","severity","confidence","country","sourceCount","firstSeenAt","lastSeenAt" FROM "Signal" WHERE "id"=$1`, signalID).
		Scan(&s.ID, &s.Title, &s.Summary, &s.Status, &s.Severity, &s.Confidence, &s.Country, &s.SourceCount, &s.FirstSeenAt, &s.LastSeenAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := d.Pool.Query(ctx,
		`SELECT tt."code" FROM "SignalTag" st JOIN "TaxonomyTag" tt ON tt."id"=st."tagId" WHERE st."signalId"=$1 ORDER BY st."tagId" ASC`, signalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		s.TagCodes = append(s.TagCodes, c)
	}
	return &s, rows.Err()
}

// EnabledSubscription is a subscription eligible for delivery matching.
type EnabledSubscription struct {
	ID      string
	Channel string
	Filter  []byte
	Config  []byte
}

// EnabledSubscriptions returns all enabled subscriptions.
func (d *DB) EnabledSubscriptions(ctx context.Context) ([]EnabledSubscription, error) {
	rows, err := d.Pool.Query(ctx, `SELECT "id","channel","filter","config" FROM "Subscription" WHERE "enabled"=true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EnabledSubscription
	for rows.Next() {
		var s EnabledSubscription
		if err := rows.Scan(&s.ID, &s.Channel, &s.Filter, &s.Config); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CreateDeliveryIfNew inserts a PENDING delivery; returns "" if (sub,signal)
// already exists (unique violation).
func (d *DB) CreateDeliveryIfNew(ctx context.Context, subID, signalID, channel string, payload []byte) (string, error) {
	id := cuid.New()
	var got string
	err := d.Pool.QueryRow(ctx,
		`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","payload","status")
		 VALUES ($1,$2,$3,$4::"DeliveryChannel",$5,'PENDING')
		 ON CONFLICT ("subscriptionId","signalId") DO NOTHING RETURNING "id"`,
		id, subID, signalID, channel, payload).Scan(&got)
	if err == pgx.ErrNoRows {
		return "", nil // conflict: already queued/delivered
	}
	if err != nil {
		return "", err
	}
	return got, nil
}

// DeliveryForSend holds the fields needed to send a delivery.
type DeliveryForSend struct {
	ID                 string
	Status             string
	Channel            string
	Attempts           int
	Payload            []byte
	SubscriptionConfig []byte
}

// LoadDeliveryForSend loads a delivery joined to its subscription config (nil if absent).
func (d *DB) LoadDeliveryForSend(ctx context.Context, deliveryID string) (*DeliveryForSend, error) {
	var dd DeliveryForSend
	err := d.Pool.QueryRow(ctx,
		`SELECT de."id",de."status",de."channel",de."attempts",de."payload",s."config"
		 FROM "DeliveryEvent" de JOIN "Subscription" s ON s."id"=de."subscriptionId" WHERE de."id"=$1`, deliveryID).
		Scan(&dd.ID, &dd.Status, &dd.Channel, &dd.Attempts, &dd.Payload, &dd.SubscriptionConfig)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &dd, nil
}

// IncrementDeliveryAttempts bumps the attempts counter.
func (d *DB) IncrementDeliveryAttempts(ctx context.Context, id string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE "DeliveryEvent" SET "attempts"="attempts"+1 WHERE "id"=$1`, id)
	return err
}

// MarkDeliverySent sets a delivery to SENT.
func (d *DB) MarkDeliverySent(ctx context.Context, id string, at time.Time) error {
	_, err := d.Pool.Exec(ctx,
		`UPDATE "DeliveryEvent" SET "status"='SENT',"deliveredAt"=$2,"errorMessage"=NULL WHERE "id"=$1`, id, at)
	return err
}

// MarkDeliveryFailed sets a delivery to a failed state with a message.
func (d *DB) MarkDeliveryFailed(ctx context.Context, id, status string, at time.Time, msg string) error {
	_, err := d.Pool.Exec(ctx,
		`UPDATE "DeliveryEvent" SET "status"=$2::"DeliveryStatus","failedAt"=$3,"errorMessage"=$4 WHERE "id"=$1`,
		id, status, at, msg)
	return err
}
