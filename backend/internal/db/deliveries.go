package db

import (
	"context"
	"time"
)

// DeliveryEvent mirrors the Prisma DeliveryEvent model plus the includes used by
// GET /v1/deliveries (subscription + signal.title).
type DeliveryEvent struct {
	ID             string
	SubscriptionID string
	SignalID       string
	Channel        string
	Status         string
	Payload        RawJSON
	Attempts       int
	DeliveredAt    *time.Time
	FailedAt       *time.Time
	ErrorMessage   *string
	CreatedAt      time.Time
	Subscription   *Subscription
	SignalTitle    string
}

const deliveryCols = `"id","subscriptionId","signalId","channel","status","payload","attempts","deliveredAt","failedAt","errorMessage","createdAt"`

// ListDeliveries returns delivery events ordered by createdAt desc (capped at
// limit, default 50, max 200) with subscription and signal title loaded.
func (d *DB) ListDeliveries(ctx context.Context, limit int) ([]*DeliveryEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := d.Pool.Query(ctx, `SELECT `+deliveryCols+` FROM "DeliveryEvent" ORDER BY "createdAt" DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	var out []*DeliveryEvent
	for rows.Next() {
		var e DeliveryEvent
		var payload []byte
		if err := rows.Scan(&e.ID, &e.SubscriptionID, &e.SignalID, &e.Channel, &e.Status, &payload, &e.Attempts, &e.DeliveredAt, &e.FailedAt, &e.ErrorMessage, &e.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		e.Payload = RawJSON(payload)
		out = append(out, &e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, e := range out {
		sub, err := d.getSubscriptionBasic(ctx, e.SubscriptionID)
		if err != nil {
			return nil, err
		}
		e.Subscription = sub
		if err := d.Pool.QueryRow(ctx, `SELECT "title" FROM "Signal" WHERE "id"=$1`, e.SignalID).Scan(&e.SignalTitle); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (d *DB) getSubscriptionBasic(ctx context.Context, id string) (*Subscription, error) {
	var s Subscription
	var filter, config []byte
	err := d.Pool.QueryRow(ctx, `SELECT `+subscriptionCols+` FROM "Subscription" WHERE "id"=$1`, id).
		Scan(&s.ID, &s.SubscriberID, &s.Name, &s.Channel, &filter, &config, &s.Enabled, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	s.Filter = RawJSON(filter)
	s.Config = RawJSON(config)
	return &s, nil
}
