package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/logging"
	"github.com/worldsignal/backend/internal/pipeline"
)

// digestIntervalFor maps a config interval to a duration.
func digestIntervalFor(interval string) time.Duration {
	if interval == "hourly" {
		return time.Hour
	}
	return 24 * time.Hour
}

// Digester rolls queued signals for digest-mode email subscriptions into a single
// email per interval. It reuses the delivery queue (delivery.send) for the actual
// send, so digests get the same retry/dead-letter handling as every other channel.
type Digester struct {
	db      *db.DB
	workers *Workers
	tick    time.Duration
	log     *logging.Logger
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewDigester builds a digester. tick is how often it checks for due digests
// (finer than the interval; a due subscription fires on the next tick).
func NewDigester(d *db.DB, w *Workers, tick time.Duration) *Digester {
	if tick <= 0 {
		tick = time.Minute
	}
	return &Digester{db: d, workers: w, tick: tick, log: logging.New("digest")}
}

// Tick builds every due digest once. A per-subscription error never aborts the
// pass. Returns the number of digest emails enqueued. Exported for testing.
func (dg *Digester) Tick(ctx context.Context, now time.Time) (int, error) {
	pending, err := dg.db.PendingDigests(ctx)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, p := range pending {
		if !digestDue(p, now) {
			continue
		}
		interval := pipeline.DigestIntervalFromConfig(p.Config)
		deliveryID, count, err := pipeline.BuildDigest(ctx, dg.db, p.SubscriptionID, interval, now)
		if err != nil {
			dg.log.Error("build digest failed", err.Error())
			continue
		}
		if deliveryID == "" {
			continue
		}
		if err := dg.workers.EnqueueSendDelivery(deliveryID); err != nil {
			dg.log.Error("enqueue digest delivery failed", err.Error())
			continue
		}
		sent++
		dg.log.Info(fmt.Sprintf("digest built: subscription=%s signals=%d delivery=%s", p.SubscriptionID, count, deliveryID))
	}
	return sent, nil
}

// digestDue reports whether a pending digest's interval has elapsed. The clock
// starts at lastDigestAt, or the oldest queued item when never digested.
func digestDue(p db.DueDigest, now time.Time) bool {
	base := p.OldestQueuedAt
	if p.LastDigestAt != nil {
		base = *p.LastDigestAt
	}
	return !now.Before(base.Add(digestIntervalFor(pipeline.DigestIntervalFromConfig(p.Config))))
}

// Start runs the digest loop (kick once, then on the tick interval).
func (dg *Digester) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	dg.cancel = cancel
	dg.done = make(chan struct{})
	go func() {
		defer close(dg.done)
		if _, err := dg.Tick(ctx, time.Now()); err != nil {
			dg.log.Error("tick failed", err.Error())
		}
		ticker := time.NewTicker(dg.tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := dg.Tick(ctx, time.Now()); err != nil {
					dg.log.Error("tick failed", err.Error())
				}
			}
		}
	}()
}

// Stop halts the digest loop.
func (dg *Digester) Stop() {
	if dg.cancel != nil {
		dg.cancel()
		<-dg.done
		dg.cancel = nil
	}
}
