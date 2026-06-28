package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/worldsignal/backend/internal/cuid"
)

// ErrDuplicateURL is returned when a source URL already exists (unique violation).
var ErrDuplicateURL = errors.New("source url already exists")

// CreateSourceInput captures the fields accepted by createSource.
type CreateSourceInput struct {
	Name           string
	URL            string
	Type           string // defaults to RSS
	Country        *string
	Priority       int
	Credibility    float64
	CrawlFrequency int
}

// CreateSource inserts a Source applying defaults and returns the full row.
func (d *DB) CreateSource(ctx context.Context, in CreateSourceInput) (*Source, error) {
	if in.Type == "" {
		in.Type = "RSS"
	}
	id := cuid.New()
	row := d.Pool.QueryRow(ctx,
		`INSERT INTO "Source" ("id","name","url","type","country","priority","credibility","crawlFrequency","updatedAt")
		 VALUES ($1,$2,$3,$4::"SourceType",$5,$6,$7,$8,now())
		 RETURNING `+sourceColumns,
		id, in.Name, in.URL, in.Type, in.Country, in.Priority, in.Credibility, in.CrawlFrequency)
	s, err := scanSource(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateURL
		}
		return nil, err
	}
	return s, nil
}

// SetSourceEnabled flips the enabled flag and returns the updated row.
func (d *DB) SetSourceEnabled(ctx context.Context, id string, enabled bool) (*Source, error) {
	row := d.Pool.QueryRow(ctx,
		`UPDATE "Source" SET "enabled"=$2, "updatedAt"=now() WHERE "id"=$1 RETURNING `+sourceColumns,
		id, enabled)
	return scanSource(row)
}

// SourcePatch holds the optional fields of PATCH /v1/sources/:id.
type SourcePatch struct {
	Enabled        *bool
	Priority       *int
	CrawlFrequency *int
}

// UpdateSource applies a partial update and returns the updated row.
func (d *DB) UpdateSource(ctx context.Context, id string, p SourcePatch) (*Source, error) {
	sets := `"updatedAt"=now()`
	args := []any{id}
	if p.Enabled != nil {
		args = append(args, *p.Enabled)
		sets += `, "enabled"=$` + itoa(len(args))
	}
	if p.Priority != nil {
		args = append(args, *p.Priority)
		sets += `, "priority"=$` + itoa(len(args))
	}
	if p.CrawlFrequency != nil {
		args = append(args, *p.CrawlFrequency)
		sets += `, "crawlFrequency"=$` + itoa(len(args))
	}
	row := d.Pool.QueryRow(ctx, `UPDATE "Source" SET `+sets+` WHERE "id"=$1 RETURNING `+sourceColumns, args...)
	return scanSource(row)
}

// UpsertDefaultSubscriber ensures the __default__ subscriber exists.
func (d *DB) UpsertDefaultSubscriber(ctx context.Context) error {
	_, err := d.Pool.Exec(ctx,
		`INSERT INTO "Subscriber" ("id","name") VALUES ('__default__','Default Subscriber')
		 ON CONFLICT ("id") DO NOTHING`)
	return err
}

// CreateSubscriptionInput captures createSubscription fields.
type CreateSubscriptionInput struct {
	Name    string
	Channel string // defaults to WEBHOOK
	Filter  RawJSON
	Config  RawJSON
}

// CreateSubscription inserts a Subscription under the default subscriber and
// returns the scalar row.
func (d *DB) CreateSubscription(ctx context.Context, in CreateSubscriptionInput) (*Subscription, error) {
	if err := d.UpsertDefaultSubscriber(ctx); err != nil {
		return nil, err
	}
	if in.Channel == "" {
		in.Channel = "WEBHOOK"
	}
	if len(in.Filter) == 0 {
		in.Filter = RawJSON("{}")
	}
	if len(in.Config) == 0 {
		in.Config = RawJSON("{}")
	}
	id := cuid.New()
	var s Subscription
	var filter, config []byte
	err := d.Pool.QueryRow(ctx,
		`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config")
		 VALUES ($1,'__default__',$2,$3::"DeliveryChannel",$4,$5)
		 RETURNING `+subscriptionCols,
		id, in.Name, in.Channel, []byte(in.Filter), []byte(in.Config)).
		Scan(&s.ID, &s.SubscriberID, &s.Name, &s.Channel, &filter, &config, &s.Enabled, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	s.Filter = RawJSON(filter)
	s.Config = RawJSON(config)
	return &s, nil
}
