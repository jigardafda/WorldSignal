package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/worldsignal/backend/internal/crypto"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/email"
)

// EmailBranding is the chrome applied to rendered emails. Set once at startup
// (from APP_BASE_URL); AppName defaults to "WorldSignal" when empty.
var EmailBranding email.Branding

// emailSend is the SMTP send function, indirected so tests can stub it without a
// real mail server.
var emailSend = email.Send

// emailConfig is the EMAIL-channel portion of a subscription's config JSON.
type emailConfig struct {
	ConnectorID string
	Recipients  []string
	Mode        string // "instant" (default) or "digest"
	Interval    string // "hourly" | "daily" (digest mode)
}

// rawEmailConfig tolerates `to` as either a string or an array of strings.
type rawEmailConfig struct {
	ConnectorID string          `json:"connectorId"`
	To          json.RawMessage `json:"to"`
	Mode        string          `json:"mode"`
	Interval    string          `json:"interval"`
}

func parseEmailConfig(raw []byte) emailConfig {
	var r rawEmailConfig
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &r)
	}
	c := emailConfig{
		ConnectorID: strings.TrimSpace(r.ConnectorID),
		Mode:        strings.ToLower(strings.TrimSpace(r.Mode)),
		Interval:    strings.ToLower(strings.TrimSpace(r.Interval)),
	}
	c.Recipients = parseRecipientsJSON(r.To)
	return c
}

func parseRecipientsJSON(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		return email.ParseRecipients(one)
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		return email.ParseRecipients(strings.Join(many, ","))
	}
	return nil
}

// isDigestConfig reports whether an EMAIL subscription is configured for digests.
func isDigestConfig(raw []byte) bool {
	return parseEmailConfig(raw).Mode == "digest"
}

// DigestIntervalFromConfig returns the normalized digest interval ("hourly" |
// "daily") from a subscription config blob (defaults to daily).
func DigestIntervalFromConfig(raw []byte) string {
	if iv := parseEmailConfig(raw).Interval; iv == "hourly" {
		return "hourly"
	}
	return "daily"
}

// sendEmailDelivery resolves the connector, decrypts its secret, renders the
// stored payload (instant signal or digest) and sends it. Returns the send error
// (nil on success); the caller records the terminal delivery state.
func sendEmailDelivery(ctx context.Context, d *db.DB, secret string, del *db.DeliveryForSend, _ time.Time) error {
	cfg := parseEmailConfig(del.SubscriptionConfig)
	if len(cfg.Recipients) == 0 {
		return fmt.Errorf("no recipients configured (set config.to)")
	}
	conn, err := d.ResolveEmailConnector(ctx, cfg.ConnectorID)
	if err != nil {
		return err
	}
	if conn == nil {
		return fmt.Errorf("no email connector configured (add one under Connectors and set it active)")
	}
	password := ""
	if conn.SecretCiphertext != "" {
		password, err = crypto.Decrypt(secret, conn.SecretCiphertext)
		if err != nil {
			return fmt.Errorf("could not decrypt connector secret: %w", err)
		}
	}
	smtp := email.SMTPConfig{
		Host: conn.Host, Port: conn.Port, Security: email.Security(conn.Security),
		Username: conn.Username, Password: password,
		FromEmail: conn.FromEmail, FromName: conn.FromName,
	}
	subject, text, html := renderDeliveryEmail(del.Payload)
	return emailSend(ctx, smtp, email.Message{To: cfg.Recipients, Subject: subject, Text: text, HTML: html})
}

// renderDeliveryEmail turns a stored delivery payload into a rendered email. It
// handles both the instant envelope (event_type "signal.published") and the
// digest envelope (event_type "signal.digest").
func renderDeliveryEmail(payload []byte) (subject, text, html string) {
	var env struct {
		EventType string          `json:"event_type"`
		Data      json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(payload, &env)

	if env.EventType == "signal.digest" {
		var d struct {
			Interval string           `json:"interval"`
			Signals  []digestCardJSON `json:"signals"`
		}
		_ = json.Unmarshal(env.Data, &d)
		cards := make([]email.SignalCard, len(d.Signals))
		for i, s := range d.Signals {
			cards[i] = s.card()
		}
		return email.RenderDigest(cards, d.Interval, EmailBranding)
	}

	var data instantCardJSON
	_ = json.Unmarshal(env.Data, &data)
	return email.RenderSignal(data.card(), EmailBranding)
}

// instantCardJSON maps the instant envelope's data block to a render card.
type instantCardJSON struct {
	SignalID    string   `json:"signal_id"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Severity    string   `json:"severity"`
	Country     *string  `json:"country"`
	Tags        []string `json:"tags"`
	SourceCount int      `json:"source_count"`
}

func (c instantCardJSON) card() email.SignalCard {
	return email.SignalCard{
		ID: c.SignalID, Title: c.Title, Summary: c.Summary, Severity: c.Severity,
		Country: deref(c.Country), Tags: c.Tags, SourceCount: c.SourceCount,
	}
}

// digestCardJSON maps one signal inside a digest envelope to a render card.
type digestCardJSON struct {
	SignalID    string   `json:"signal_id"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Severity    string   `json:"severity"`
	Country     *string  `json:"country"`
	Tags        []string `json:"tags"`
	SourceCount int      `json:"source_count"`
	Link        *string  `json:"link"`
	LastSeenAt  string   `json:"last_seen_at"`
}

func (c digestCardJSON) card() email.SignalCard {
	return email.SignalCard{
		ID: c.SignalID, Title: c.Title, Summary: c.Summary, Severity: c.Severity,
		Country: deref(c.Country), Tags: c.Tags, SourceCount: c.SourceCount,
		Link: deref(c.Link), WhenText: relTime(c.LastSeenAt),
	}
}

// BuildDigest collects a subscription's queued signals into a single rollup
// delivery. It returns the new delivery id ("" when nothing was pending) and the
// signal count. The caller enqueues delivery.send for a non-empty id.
func BuildDigest(ctx context.Context, d *db.DB, subID, interval string, now time.Time) (string, int, error) {
	return d.BuildDigestDelivery(ctx, subID, now, func(sigs []db.DigestSignal) (string, []byte) {
		// BuildDigestDelivery only invokes this with a non-empty batch.
		rep := sigs[0].ID // newest first → unique per digest (removed from queue after)
		return rep, buildDigestEnvelope(subID, interval, sigs, now)
	})
}

func buildDigestEnvelope(subID, interval string, sigs []db.DigestSignal, now time.Time) []byte {
	type sigJSON struct {
		SignalID    string   `json:"signal_id"`
		Title       string   `json:"title"`
		Summary     string   `json:"summary"`
		Severity    string   `json:"severity"`
		Country     *string  `json:"country"`
		Tags        []string `json:"tags"`
		SourceCount int      `json:"source_count"`
		Link        *string  `json:"link"`
		FirstSeenAt string   `json:"first_seen_at"`
		LastSeenAt  string   `json:"last_seen_at"`
	}
	items := make([]sigJSON, len(sigs))
	for i, s := range sigs {
		tags := s.Tags
		if tags == nil {
			tags = []string{}
		}
		items[i] = sigJSON{
			SignalID: s.ID, Title: s.Title, Summary: s.Summary, Severity: s.Severity,
			Country: s.Country, Tags: tags, SourceCount: s.SourceCount, Link: s.Link,
			FirstSeenAt: s.FirstSeenAt.UTC().Format(isoLayout), LastSeenAt: s.LastSeenAt.UTC().Format(isoLayout),
		}
	}
	env := map[string]any{
		"schema_version":  "2026-06-01",
		"event_type":      "signal.digest",
		"event_id":        fmt.Sprintf("dig_%s_%s", subID, sigs[0].ID),
		"created_at":      now.UTC().Format(isoLayout),
		"subscription_id": subID,
		"data": map[string]any{
			"interval": interval,
			"count":    len(sigs),
			"signals":  items,
		},
	}
	b, _ := json.Marshal(env)
	return b
}

// relTime renders an ISO timestamp as a compact relative label for emails.
func relTime(iso string) string {
	if iso == "" {
		return ""
	}
	t, err := time.Parse(isoLayout, iso)
	if err != nil {
		if t, err = time.Parse(time.RFC3339, iso); err != nil {
			return ""
		}
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
