package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/worldsignal/backend/internal/cuid"
)

// EmailConnector is an admin-managed SMTP configuration used by the EMAIL delivery
// channel. The secret (password/API key) is stored only as ciphertext; callers
// decrypt on demand. SecretLast4 is safe to display.
type EmailConnector struct {
	ID               string
	Name             string
	Provider         string
	Host             string
	Port             int
	Security         string
	Username         string
	SecretCiphertext string
	SecretLast4      string
	FromEmail        string
	FromName         string
	IsActive         bool
	Enabled          bool
	Status           string
	LastTestedAt     *time.Time
	LastError        *string
	CreatedBy        *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

const connectorCols = `"id","name","provider","host","port","security","username","secretCiphertext","secretLast4","fromEmail","fromName","isActive","enabled","status","lastTestedAt","lastError","createdBy","createdAt","updatedAt"`

func scanConnector(row pgx.Row) (*EmailConnector, error) {
	var c EmailConnector
	if err := row.Scan(&c.ID, &c.Name, &c.Provider, &c.Host, &c.Port, &c.Security, &c.Username,
		&c.SecretCiphertext, &c.SecretLast4, &c.FromEmail, &c.FromName, &c.IsActive, &c.Enabled,
		&c.Status, &c.LastTestedAt, &c.LastError, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// ListEmailConnectors returns all connectors, active first then newest.
func (d *DB) ListEmailConnectors(ctx context.Context) ([]*EmailConnector, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+connectorCols+` FROM "EmailConnector" ORDER BY "isActive" DESC, "createdAt" DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*EmailConnector{}
	for rows.Next() {
		c, err := scanConnector(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetEmailConnector returns one connector by id, or (nil, nil) if absent.
func (d *DB) GetEmailConnector(ctx context.Context, id string) (*EmailConnector, error) {
	c, err := scanConnector(d.Pool.QueryRow(ctx, `SELECT `+connectorCols+` FROM "EmailConnector" WHERE "id"=$1`, id))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// GetActiveEmailConnector returns the active, enabled connector, or (nil, nil).
func (d *DB) GetActiveEmailConnector(ctx context.Context) (*EmailConnector, error) {
	c, err := scanConnector(d.Pool.QueryRow(ctx,
		`SELECT `+connectorCols+` FROM "EmailConnector" WHERE "isActive"=true AND "enabled"=true ORDER BY "updatedAt" DESC LIMIT 1`))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// ResolveEmailConnector loads the connector a delivery should use: the one named
// by connectorID when set and enabled, otherwise the active connector. Returns
// (nil, nil) when none is available.
func (d *DB) ResolveEmailConnector(ctx context.Context, connectorID string) (*EmailConnector, error) {
	if connectorID != "" {
		c, err := d.GetEmailConnector(ctx, connectorID)
		if err != nil || c == nil {
			return c, err
		}
		if c.Enabled {
			return c, nil
		}
	}
	return d.GetActiveEmailConnector(ctx)
}

// CreateEmailConnectorInput carries the fields to persist for a new connector.
type CreateEmailConnectorInput struct {
	Name, Provider, Host, Security, Username, Ciphertext, Last4, FromEmail, FromName string
	Port                                                                             int
	CreatedBy                                                                        *string
}

// CreateEmailConnector inserts a connector (inactive, untested) and returns it.
func (d *DB) CreateEmailConnector(ctx context.Context, id string, in CreateEmailConnectorInput) (*EmailConnector, error) {
	return scanConnector(d.Pool.QueryRow(ctx, `
INSERT INTO "EmailConnector" ("id","name","provider","host","port","security","username","secretCiphertext","secretLast4","fromEmail","fromName","createdBy","updatedAt")
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,now())
RETURNING `+connectorCols,
		id, in.Name, in.Provider, in.Host, in.Port, in.Security, in.Username, in.Ciphertext, in.Last4, in.FromEmail, in.FromName, in.CreatedBy))
}

// UpdateEmailConnectorInput carries mutable connector fields. A nil pointer leaves
// the corresponding column unchanged; SecretCiphertext/Last4 are updated only when
// Ciphertext is non-nil (so editing without re-entering the secret keeps it).
type UpdateEmailConnectorInput struct {
	Name, Host, Security, Username, FromEmail, FromName *string
	Port                                                *int
	Enabled                                             *bool
	Ciphertext, Last4                                   *string
}

// UpdateEmailConnector applies a partial update and marks the connector untested
// (config changed → must be re-verified). Returns (nil,nil) if the id is unknown.
func (d *DB) UpdateEmailConnector(ctx context.Context, id string, in UpdateEmailConnectorInput) (*EmailConnector, error) {
	set := "SET \"updatedAt\"=now(),\"status\"='UNTESTED',\"lastError\"=NULL"
	args := []any{id}
	add := func(col string, v any) {
		args = append(args, v)
		set += ", \"" + col + "\"=$" + itoa(len(args))
	}
	if in.Name != nil {
		add("name", *in.Name)
	}
	if in.Host != nil {
		add("host", *in.Host)
	}
	if in.Port != nil {
		add("port", *in.Port)
	}
	if in.Security != nil {
		add("security", *in.Security)
	}
	if in.Username != nil {
		add("username", *in.Username)
	}
	if in.FromEmail != nil {
		add("fromEmail", *in.FromEmail)
	}
	if in.FromName != nil {
		add("fromName", *in.FromName)
	}
	if in.Enabled != nil {
		add("enabled", *in.Enabled)
	}
	if in.Ciphertext != nil {
		add("secretCiphertext", *in.Ciphertext)
		last4 := ""
		if in.Last4 != nil {
			last4 = *in.Last4
		}
		add("secretLast4", last4)
	}
	c, err := scanConnector(d.Pool.QueryRow(ctx, `UPDATE "EmailConnector" `+set+` WHERE "id"=$1 RETURNING `+connectorCols, args...))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// SetActiveEmailConnector makes one connector the sole active connector.
func (d *DB) SetActiveEmailConnector(ctx context.Context, id string) (*EmailConnector, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists string
	if err := tx.QueryRow(ctx, `SELECT "id" FROM "EmailConnector" WHERE "id"=$1`, id).Scan(&exists); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE "EmailConnector" SET "isActive"=false,"updatedAt"=now() WHERE "id"<>$1 AND "isActive"=true`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE "EmailConnector" SET "isActive"=true,"enabled"=true,"updatedAt"=now() WHERE "id"=$1`, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return d.GetEmailConnector(ctx, id)
}

// UpdateEmailConnectorStatus records the outcome of a connection test.
func (d *DB) UpdateEmailConnectorStatus(ctx context.Context, id, status string, errMsg *string) error {
	_, err := d.Pool.Exec(ctx,
		`UPDATE "EmailConnector" SET "status"=$2,"lastError"=$3,"lastTestedAt"=now(),"updatedAt"=now() WHERE "id"=$1`,
		id, status, errMsg)
	return err
}

// DeleteEmailConnector removes a connector, returning whether a row was deleted.
func (d *DB) DeleteEmailConnector(ctx context.Context, id string) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM "EmailConnector" WHERE "id"=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ---- Digest queue ----

// QueueDigestItem records that a signal matched a digest-mode subscription. It is
// idempotent per (subscription, signal).
func (d *DB) QueueDigestItem(ctx context.Context, subID, signalID string, now time.Time) error {
	_, err := d.Pool.Exec(ctx,
		`INSERT INTO "DigestQueue" ("subscriptionId","signalId","queuedAt") VALUES ($1,$2,$3)
		 ON CONFLICT ("subscriptionId","signalId") DO NOTHING`, subID, signalID, now)
	return err
}

// DueDigest describes a digest-mode subscription with pending queued signals.
type DueDigest struct {
	SubscriptionID string
	Config         []byte
	LastDigestAt   *time.Time
	OldestQueuedAt time.Time
	QueuedCount    int
}

// PendingDigests returns enabled EMAIL subscriptions that have queued signals.
// Presence of queued items implies the subscription is in digest mode (only the
// digest path enqueues). The caller decides due-ness from the config interval.
func (d *DB) PendingDigests(ctx context.Context) ([]DueDigest, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT s."id", s."config", s."lastDigestAt", min(dq."queuedAt"), count(*)
FROM "Subscription" s JOIN "DigestQueue" dq ON dq."subscriptionId"=s."id"
WHERE s."enabled"=true AND s."channel"='EMAIL'
GROUP BY s."id", s."config", s."lastDigestAt"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DueDigest
	for rows.Next() {
		var dd DueDigest
		if err := rows.Scan(&dd.SubscriptionID, &dd.Config, &dd.LastDigestAt, &dd.OldestQueuedAt, &dd.QueuedCount); err != nil {
			return nil, err
		}
		out = append(out, dd)
	}
	return out, rows.Err()
}

// DigestSignal is the render-ready view of a queued signal for a digest email.
type DigestSignal struct {
	ID          string
	Title       string
	Summary     string
	Severity    string
	Country     *string
	SourceCount int
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	Link        *string
	Tags        []string
}

// digestBatchLimit bounds one digest email so a huge backlog can't build a
// pathological message; the remainder rolls into the next interval.
const digestBatchLimit = 200

// BuildDigestDelivery atomically collects the queued signals for a subscription,
// hands them to build() to produce a delivery payload, creates a single EMAIL
// DeliveryEvent, clears the queue and stamps lastDigestAt. It is safe under
// concurrent schedulers (queue rows are locked). Returns the new delivery id (""
// when there was nothing to send) and the number of signals included.
//
// build receives the collected signals (newest first) and returns the id of the
// representative signal (the DeliveryEvent's signalId) and the payload bytes;
// returning empty payload skips creating a delivery but still drains the queue.
func (d *DB) BuildDigestDelivery(ctx context.Context, subID string, now time.Time,
	build func([]DigestSignal) (repSignalID string, payload []byte)) (string, int, error) {

	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
SELECT s."id", s."title", s."summary", s."severity", s."country", s."sourceCount", s."firstSeenAt", s."lastSeenAt",
       (SELECT a."canonicalUrl" FROM "SignalArticle" sa JOIN "Article" a ON a."id"=sa."articleId"
        WHERE sa."signalId"=s."id" AND a."canonicalUrl" IS NOT NULL
        ORDER BY (sa."relationType"='PRIMARY') DESC, a."publishedAt" DESC NULLS LAST LIMIT 1) AS link
FROM "DigestQueue" dq JOIN "Signal" s ON s."id"=dq."signalId"
WHERE dq."subscriptionId"=$1
ORDER BY s."lastSeenAt" DESC
LIMIT $2
FOR UPDATE OF dq`, subID, digestBatchLimit)
	if err != nil {
		return "", 0, err
	}
	var sigs []DigestSignal
	var ids []string
	for rows.Next() {
		var s DigestSignal
		if err := rows.Scan(&s.ID, &s.Title, &s.Summary, &s.Severity, &s.Country, &s.SourceCount,
			&s.FirstSeenAt, &s.LastSeenAt, &s.Link); err != nil {
			rows.Close()
			return "", 0, err
		}
		sigs = append(sigs, s)
		ids = append(ids, s.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", 0, err
	}

	// Always advance the interval clock so an empty/queued-again subscription
	// doesn't fire repeatedly.
	if _, err := tx.Exec(ctx, `UPDATE "Subscription" SET "lastDigestAt"=$2 WHERE "id"=$1`, subID, now); err != nil {
		return "", 0, err
	}
	if len(sigs) == 0 {
		return "", 0, tx.Commit(ctx)
	}

	// Attach tags for the collected signals.
	if err := attachDigestTags(ctx, tx, sigs, ids); err != nil {
		return "", 0, err
	}

	repID, payload := build(sigs)
	var deliveryID string
	if len(payload) > 0 && repID != "" {
		err := tx.QueryRow(ctx,
			`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","payload","status")
			 VALUES ($1,$2,$3,'EMAIL',$4,'PENDING')
			 ON CONFLICT ("subscriptionId","signalId") DO NOTHING RETURNING "id"`,
			cuid.New(), subID, repID, payload).Scan(&deliveryID)
		if err != nil && err != pgx.ErrNoRows {
			return "", 0, err
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM "DigestQueue" WHERE "subscriptionId"=$1 AND "signalId"=ANY($2)`, subID, ids); err != nil {
		return "", 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", 0, err
	}
	return deliveryID, len(sigs), nil
}

func attachDigestTags(ctx context.Context, tx pgx.Tx, sigs []DigestSignal, ids []string) error {
	rows, err := tx.Query(ctx,
		`SELECT st."signalId", tt."code" FROM "SignalTag" st JOIN "TaxonomyTag" tt ON tt."id"=st."tagId"
		 WHERE st."signalId"=ANY($1) ORDER BY st."tagId" ASC`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	byID := map[string][]string{}
	for rows.Next() {
		var sid, code string
		if err := rows.Scan(&sid, &code); err != nil {
			return err
		}
		byID[sid] = append(byID[sid], code)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range sigs {
		sigs[i].Tags = byID[sigs[i].ID]
	}
	return nil
}
