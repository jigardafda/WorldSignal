package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

var pgxErrNoRows = pgx.ErrNoRows

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
	AccountID     string
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

const subscriptionCols = `"id","accountId","subscriberId","name","channel","filter","config","enabled","createdAt"`

// scanSubscription reads a subscription row in subscriptionCols order.
func scanSubscription(row interface {
	Scan(dest ...any) error
}) (*Subscription, error) {
	var s Subscription
	var filter, config []byte
	if err := row.Scan(&s.ID, &s.AccountID, &s.SubscriberID, &s.Name, &s.Channel, &filter, &config, &s.Enabled, &s.CreatedAt); err != nil {
		return nil, err
	}
	s.Filter = RawJSON(filter)
	s.Config = RawJSON(config)
	return &s, nil
}

// SubscriptionAccountID returns the owning account of a subscription, or ("",nil)
// if the subscription does not exist. Used for tenant ownership (IDOR) checks.
func (d *DB) SubscriptionAccountID(ctx context.Context, id string) (string, error) {
	var acct string
	err := d.Pool.QueryRow(ctx, `SELECT "accountId" FROM "Subscription" WHERE "id"=$1`, id).Scan(&acct)
	if err == pgxErrNoRows {
		return "", nil
	}
	return acct, err
}

// ListSubscriptionsBasicByAccount returns an account's subscriptions (scalar).
func (d *DB) ListSubscriptionsBasicByAccount(ctx context.Context, accountID string) ([]*Subscription, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+subscriptionCols+` FROM "Subscription" WHERE "accountId"=$1 ORDER BY "createdAt" DESC LIMIT 500`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	subs := []*Subscription{}
	for rows.Next() {
		s, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

// ListSubscriptionsByAccount returns an account's subscriptions with the
// subscriber + delivery-count includes (the account-scoped REST list).
func (d *DB) ListSubscriptionsByAccount(ctx context.Context, accountID string) ([]*Subscription, error) {
	subs, err := d.ListSubscriptionsBasicByAccount(ctx, accountID)
	if err != nil {
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

// ListSubscriptions returns subscriptions ordered by createdAt desc, with the
// subscriber and delivery count loaded (matching the REST include).
func (d *DB) ListSubscriptions(ctx context.Context) ([]*Subscription, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+subscriptionCols+` FROM "Subscription" ORDER BY "createdAt" DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	var subs []*Subscription
	for rows.Next() {
		s, err := scanSubscription(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		subs = append(subs, s)
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
	rows, err := d.Pool.Query(ctx, `SELECT `+subscriptionCols+` FROM "Subscription" ORDER BY "createdAt" DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []*Subscription
	for rows.Next() {
		s, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, s)
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
