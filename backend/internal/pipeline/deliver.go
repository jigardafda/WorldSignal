package pipeline

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/worldsignal/backend/internal/db"
)

const isoLayout = "2006-01-02T15:04:05.000Z"

var severityRank = map[string]int{"LOW": 0, "MEDIUM": 1, "HIGH": 2, "CRITICAL": 3}

type subscriptionFilter struct {
	Tags          []string `json:"tags"`
	Countries     []string `json:"countries"`
	MinConfidence *float64 `json:"minConfidence"`
	MinSeverity   *string  `json:"minSeverity"`
}

func matchesFilter(f subscriptionFilter, s *db.SignalForMatch) bool {
	if f.MinConfidence != nil && s.Confidence < *f.MinConfidence {
		return false
	}
	if f.MinSeverity != nil && severityRank[s.Severity] < severityRank[*f.MinSeverity] {
		return false
	}
	if len(f.Countries) > 0 {
		if s.Country == nil || !contains(f.Countries, *s.Country) {
			return false
		}
	}
	if len(f.Tags) > 0 {
		hit := false
		for _, code := range s.TagCodes {
			for _, want := range f.Tags {
				if code == want || hasPrefix(code, want+".") {
					hit = true
				}
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

// MatchSubscriptions matches a signal against enabled subscriptions and creates
// PENDING delivery rows. Mirrors matchSubscriptions in deliver.ts.
func MatchSubscriptions(ctx context.Context, d *db.DB, signalID string, now time.Time) ([]string, error) {
	sig, err := d.LoadSignalForMatch(ctx, signalID)
	if err != nil || sig == nil {
		return nil, err
	}
	subs, err := d.EnabledSubscriptions(ctx)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, sub := range subs {
		var f subscriptionFilter
		if len(sub.Filter) > 0 {
			_ = json.Unmarshal(sub.Filter, &f)
		}
		if !matchesFilter(f, sig) {
			continue
		}
		payload := buildEnvelope(sub.ID, sig, now)
		id, err := d.CreateDeliveryIfNew(ctx, sub.ID, signalID, sub.Channel, payload)
		if err != nil {
			return nil, err
		}
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func buildEnvelope(subscriptionID string, s *db.SignalForMatch, now time.Time) []byte {
	tags := s.TagCodes
	if tags == nil {
		tags = []string{}
	}
	env := struct {
		SchemaVersion  string `json:"schema_version"`
		EventType      string `json:"event_type"`
		EventID        string `json:"event_id"`
		CreatedAt      string `json:"created_at"`
		SubscriptionID string `json:"subscription_id"`
		Data           struct {
			SignalID    string   `json:"signal_id"`
			Title       string   `json:"title"`
			Summary     string   `json:"summary"`
			Status      string   `json:"status"`
			Severity    string   `json:"severity"`
			Confidence  float64  `json:"confidence"`
			Country     *string  `json:"country"`
			Tags        []string `json:"tags"`
			SourceCount int      `json:"source_count"`
			FirstSeenAt string   `json:"first_seen_at"`
			LastSeenAt  string   `json:"last_seen_at"`
		} `json:"data"`
	}{
		SchemaVersion:  "2026-06-01",
		EventType:      "signal.published",
		EventID:        fmt.Sprintf("evt_%s_%s", s.ID, subscriptionID),
		CreatedAt:      now.UTC().Format(isoLayout),
		SubscriptionID: subscriptionID,
	}
	env.Data.SignalID = s.ID
	env.Data.Title = s.Title
	env.Data.Summary = s.Summary
	env.Data.Status = s.Status
	env.Data.Severity = s.Severity
	env.Data.Confidence = s.Confidence
	env.Data.Country = s.Country
	env.Data.Tags = tags
	env.Data.SourceCount = s.SourceCount
	env.Data.FirstSeenAt = s.FirstSeenAt.UTC().Format(isoLayout)
	env.Data.LastSeenAt = s.LastSeenAt.UTC().Format(isoLayout)

	b, _ := json.Marshal(env)
	return b
}

// SignPayload mirrors signPayload in deliver.ts.
func SignPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// SendDelivery sends one delivery. Mirrors sendDelivery in deliver.ts; `now` is
// injected and `client` allows test stubs.
func SendDelivery(ctx context.Context, d *db.DB, client *http.Client, secret, deliveryID string, isFinalAttempt bool, now time.Time) error {
	del, err := d.LoadDeliveryForSend(ctx, deliveryID)
	if err != nil || del == nil {
		return err
	}
	if del.Status == "SENT" {
		return nil
	}
	if err := d.IncrementDeliveryAttempts(ctx, deliveryID); err != nil {
		return err
	}

	if del.Channel == "POLLING" {
		return d.MarkDeliverySent(ctx, deliveryID, now)
	}

	var config struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(del.SubscriptionConfig, &config)
	if config.URL == "" {
		return d.MarkDeliveryFailed(ctx, deliveryID, "FAILED", now, "no webhook url configured")
	}

	// Compact the stored payload to match JSON.stringify(payload) byte-for-byte.
	var body bytes.Buffer
	if err := json.Compact(&body, del.Payload); err != nil {
		return err
	}
	var env struct {
		EventID string `json:"event_id"`
	}
	_ = json.Unmarshal(del.Payload, &env)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.URL, bytes.NewReader(body.Bytes()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-WorldSignal-Event-Id", env.EventID)
	req.Header.Set("X-WorldSignal-Signature", SignPayload(secret, body.Bytes()))
	req.Header.Set("X-WorldSignal-Timestamp", now.UTC().Format(isoLayout))
	req.Header.Set("X-WorldSignal-Attempt", itoa(del.Attempts+1))

	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return d.MarkDeliverySent(ctx, deliveryID, now)
		}
		err = fmt.Errorf("webhook responded %d", resp.StatusCode)
	}

	status := "RETRYING"
	if isFinalAttempt {
		status = "DEAD_LETTERED"
	}
	if merr := d.MarkDeliveryFailed(ctx, deliveryID, status, now, err.Error()); merr != nil {
		return merr
	}
	if !isFinalAttempt {
		return err // surface so the queue retries
	}
	return nil
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
