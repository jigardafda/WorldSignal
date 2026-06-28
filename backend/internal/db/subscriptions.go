package db

import (
	"context"
	"time"
)

// Subscriber mirrors the Prisma Subscriber model (+ a subscription count for lists).
type Subscriber struct {
	ID                string
	Name              string
	Status            string
	CreatedAt         time.Time
	SubscriptionCount int
}

// Subscription mirrors the Prisma Subscription model plus the includes used by
// GET /v1/subscriptions (subscriber + delivery count).
type Subscription struct {
	ID            string
	SubscriberID  string
	Name          string
	Channel       string
	Filter        RawJSON
	Config        RawJSON
	Enabled       bool
	CreatedAt     time.Time
	Subscriber    *Subscriber
	DeliveryCount int
}

const subscriptionCols = `"id","subscriberId","name","channel","filter","config","enabled","createdAt"`

// ListSubscriptions returns subscriptions ordered by createdAt desc, with the
// subscriber and delivery count loaded (matching the REST include).
func (d *DB) ListSubscriptions(ctx context.Context) ([]*Subscription, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+subscriptionCols+` FROM "Subscription" ORDER BY "createdAt" DESC`)
	if err != nil {
		return nil, err
	}
	var subs []*Subscription
	for rows.Next() {
		var s Subscription
		var filter, config []byte
		if err := rows.Scan(&s.ID, &s.SubscriberID, &s.Name, &s.Channel, &filter, &config, &s.Enabled, &s.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		s.Filter = RawJSON(filter)
		s.Config = RawJSON(config)
		subs = append(subs, &s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, s := range subs {
		sub, err := d.getSubscriber(ctx, s.SubscriberID)
		if err != nil {
			return nil, err
		}
		s.Subscriber = sub
		if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "DeliveryEvent" WHERE "subscriptionId"=$1`, s.ID).Scan(&s.DeliveryCount); err != nil {
			return nil, err
		}
	}
	return subs, nil
}

// ListSubscriptionsBasic returns subscriptions ordered by createdAt desc without
// the subscriber/_count includes (used by the GraphQL subscriptions query).
func (d *DB) ListSubscriptionsBasic(ctx context.Context) ([]*Subscription, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+subscriptionCols+` FROM "Subscription" ORDER BY "createdAt" DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []*Subscription
	for rows.Next() {
		var s Subscription
		var filter, config []byte
		if err := rows.Scan(&s.ID, &s.SubscriberID, &s.Name, &s.Channel, &filter, &config, &s.Enabled, &s.CreatedAt); err != nil {
			return nil, err
		}
		s.Filter = RawJSON(filter)
		s.Config = RawJSON(config)
		subs = append(subs, &s)
	}
	return subs, rows.Err()
}

func (d *DB) getSubscriber(ctx context.Context, id string) (*Subscriber, error) {
	var s Subscriber
	err := d.Pool.QueryRow(ctx, `SELECT "id","name","status","createdAt" FROM "Subscriber" WHERE "id"=$1`, id).
		Scan(&s.ID, &s.Name, &s.Status, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
