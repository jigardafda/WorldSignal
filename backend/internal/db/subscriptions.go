package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

var pgxErrNoRows = pgx.ErrNoRows

// Subscription mirrors the Subscription model plus the includes used by
// GET /v1/subscriptions (owning account + delivery count). Subscriptions are
// owned by an Account (the tenant).
type Subscription struct {
	ID            string
	AccountID     string
	Name          string
	Channel       string
	Filter        RawJSON
	Config        RawJSON
	Enabled       bool
	CreatedAt     time.Time
	Account       *Account
	DeliveryCount int
}

const subscriptionCols = `"id","accountId","name","channel","filter","config","enabled","createdAt"`

// scanSubscription reads a subscription row in subscriptionCols order.
func scanSubscription(row interface {
	Scan(dest ...any) error
}) (*Subscription, error) {
	var s Subscription
	var filter, config []byte
	if err := row.Scan(&s.ID, &s.AccountID, &s.Name, &s.Channel, &filter, &config, &s.Enabled, &s.CreatedAt); err != nil {
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

// ListSubscriptionsByAccount returns an account's subscriptions with the owning
// account + delivery-count includes (the account-scoped REST list).
func (d *DB) ListSubscriptionsByAccount(ctx context.Context, accountID string) ([]*Subscription, error) {
	subs, err := d.ListSubscriptionsBasicByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if len(subs) == 0 {
		return subs, nil
	}
	// All rows share the same owning account; load it once.
	acct, err := d.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	for _, s := range subs {
		s.Account = acct
		if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "DeliveryEvent" WHERE "subscriptionId"=$1`, s.ID).Scan(&s.DeliveryCount); err != nil {
			return nil, err
		}
	}
	return subs, nil
}

// ListSubscriptionsBasic returns all subscriptions ordered by createdAt desc
// without includes (the operator cross-tenant GraphQL query).
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
