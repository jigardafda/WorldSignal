package db

import (
	"context"
	"encoding/json"

	"github.com/worldsignal/backend/internal/relevance"
)

// CandidateSignals loads recent signals (last sinceHours, newest first, capped at
// limit) projected to relevance.Signal — including their category tags and entity
// names from SignalAttribute — ready to be scored for a profile.
func (d *DB) CandidateSignals(ctx context.Context, sinceHours, limit int) ([]relevance.Signal, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	if sinceHours <= 0 {
		sinceHours = 72
	}
	const q = `
SELECT s."id", s."title", COALESCE(s."summary",''), COALESCE(s."eventType",''),
       COALESCE(s."country",''), COALESCE(s."region",''), COALESCE(s."sentiment"::text,''),
       COALESCE(s."influence"::text,''), COALESCE(s."severity"::text,''),
       COALESCE(s."relevance",0), COALESCE(s."confidence",0),
       EXTRACT(EPOCH FROM (now() - s."lastSeenAt"))/3600.0 AS age_hours,
       COALESCE((SELECT array_agg(a."valueCode") FROM "SignalAttribute" a
                 WHERE a."signalId"=s."id" AND a."key"='category'), ARRAY[]::text[]) AS tags,
       COALESCE((SELECT array_agg(a."valueText") FROM "SignalAttribute" a
                 WHERE a."signalId"=s."id" AND a."key"='entity' AND a."valueText" <> ''), ARRAY[]::text[]) AS entities
FROM "Signal" s
WHERE s."lastSeenAt" >= now() - make_interval(hours => $1)
ORDER BY s."lastSeenAt" DESC
LIMIT $2`
	rows, err := d.Pool.Query(ctx, q, sinceHours, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []relevance.Signal
	for rows.Next() {
		var s relevance.Signal
		if err := rows.Scan(&s.ID, &s.Title, &s.Summary, &s.EventType, &s.Country, &s.Region,
			&s.Sentiment, &s.Influence, &s.Severity, &s.Relevance, &s.Confidence, &s.AgeHours,
			&s.Tags, &s.Entities); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LoadProfile builds a relevance.Profile for a subscription: its weighted
// interests (the interests column) plus the filter's keyword, if any.
func (d *DB) LoadProfile(ctx context.Context, subscriptionID string) (relevance.Profile, error) {
	var interestsRaw, filterRaw []byte
	err := d.Pool.QueryRow(ctx,
		`SELECT "interests", "filter" FROM "Subscription" WHERE "id"=$1`, subscriptionID).
		Scan(&interestsRaw, &filterRaw)
	if err != nil {
		return relevance.Profile{}, err
	}
	p := relevance.Profile{Interests: map[string]float64{}}
	if len(interestsRaw) > 0 {
		_ = json.Unmarshal(interestsRaw, &p.Interests)
	}
	// The visual filter's free-text keyword doubles as a relevance keyword.
	if len(filterRaw) > 0 {
		var f struct {
			Keyword string `json:"keyword"`
		}
		if json.Unmarshal(filterRaw, &f) == nil && f.Keyword != "" {
			p.Keywords = append(p.Keywords, f.Keyword)
		}
	}
	return p, nil
}

// SetSubscriptionInterests replaces a subscription's weighted interest graph.
func (d *DB) SetSubscriptionInterests(ctx context.Context, subscriptionID string, interests map[string]float64) error {
	if interests == nil {
		interests = map[string]float64{}
	}
	raw, err := json.Marshal(interests)
	if err != nil {
		return err
	}
	_, err = d.Pool.Exec(ctx,
		`UPDATE "Subscription" SET "interests"=$2 WHERE "id"=$1`, subscriptionID, raw)
	return err
}

// RecordFeedback logs a subscriber's reaction to a signal (open/up/down),
// upserting so repeated actions are idempotent.
func (d *DB) RecordFeedback(ctx context.Context, subscriptionID, signalID, action string) error {
	_, err := d.Pool.Exec(ctx,
		`INSERT INTO "SignalFeedback" ("subscriptionId","signalId","action")
		 VALUES ($1,$2,$3)
		 ON CONFLICT ("subscriptionId","signalId","action") DO UPDATE SET "createdAt"=now()`,
		subscriptionID, signalID, action)
	return err
}

// RankedFeed loads recent candidate signals and returns them ranked for the
// given subscription's profile, capped at limit.
func (d *DB) RankedFeed(ctx context.Context, subscriptionID string, sinceHours, limit int) ([]relevance.Scored, error) {
	profile, err := d.LoadProfile(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	cands, err := d.CandidateSignals(ctx, sinceHours, maxCandidates(limit))
	if err != nil {
		return nil, err
	}
	ranked := relevance.Rank(profile, cands)
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked, nil
}

// maxCandidates fetches a wider pool than the requested page so ranking has
// something to choose from.
func maxCandidates(limit int) int {
	pool := limit * 6
	if pool < 300 {
		pool = 300
	}
	if pool > 2000 {
		pool = 2000
	}
	return pool
}
