package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/logging"
)

// maxEnqueuePerTick caps how many fetches one tick enqueues so a large backlog
// drains over a few ticks instead of flooding the queue at once.
const maxEnqueuePerTick = 2000

// Scheduler enqueues fetches for sources whose crawl interval has elapsed.
type Scheduler struct {
	db      *db.DB
	workers *Workers
	tick    time.Duration
	log     *logging.Logger
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewScheduler builds a scheduler with the given tick interval.
func NewScheduler(d *db.DB, w *Workers, tick time.Duration) *Scheduler {
	return &Scheduler{db: d, workers: w, tick: tick, log: logging.New("scheduler")}
}

// Tick enqueues fetches for all currently-due sources. The due filter runs in
// SQL (scales to thousands); a single source's enqueue error never aborts the
// tick. Returns the number enqueued. Exported for testing.
func (s *Scheduler) Tick(ctx context.Context, now time.Time) (int, error) {
	ids, err := s.db.ListDueSources(ctx, now, maxEnqueuePerTick)
	if err != nil {
		return 0, err
	}
	enqueued, errs := 0, 0
	for _, id := range ids {
		if err := s.workers.EnqueueFetchSource(id); err != nil {
			errs++
			s.log.Error("enqueue fetch failed", err.Error())
			continue // resilience: keep scheduling the rest
		}
		enqueued++
	}
	if len(ids) > 0 || errs > 0 {
		s.log.Info(fmt.Sprintf("scheduler tick: due=%d enqueued=%d errors=%d", len(ids), enqueued, errs))
	}
	return enqueued, nil
}

// Start runs the scheduler loop (kick once, then on the tick interval).
func (s *Scheduler) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		if _, err := s.Tick(ctx, time.Now()); err != nil {
			s.log.Error("tick failed", err.Error())
		}
		ticker := time.NewTicker(s.tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := s.Tick(ctx, time.Now()); err != nil {
					s.log.Error("tick failed", err.Error())
				}
			}
		}
	}()
}

// Stop halts the scheduler loop.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
		<-s.done
		s.cancel = nil
	}
}
