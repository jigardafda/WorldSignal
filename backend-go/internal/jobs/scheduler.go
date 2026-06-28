package jobs

import (
	"context"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/logging"
)

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

// DueSource is an enabled source eligible for a fetch.
type dueSource struct {
	ID             string
	CrawlFrequency int
	LastFetchedAt  *time.Time
}

// Tick enqueues fetches for due sources. Exported for testing.
func (s *Scheduler) Tick(ctx context.Context, now time.Time) (int, error) {
	rows, err := s.db.Pool.Query(ctx,
		`SELECT "id","crawlFrequency","lastFetchedAt" FROM "Source" WHERE "enabled"=true ORDER BY "priority" ASC`)
	if err != nil {
		return 0, err
	}
	var sources []dueSource
	for rows.Next() {
		var d dueSource
		if err := rows.Scan(&d.ID, &d.CrawlFrequency, &d.LastFetchedAt); err != nil {
			rows.Close()
			return 0, err
		}
		sources = append(sources, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	due := 0
	for _, src := range sources {
		var last time.Time
		if src.LastFetchedAt != nil {
			last = *src.LastFetchedAt
		}
		if now.Sub(last) >= time.Duration(src.CrawlFrequency)*time.Second {
			if err := s.workers.EnqueueFetchSource(src.ID); err != nil {
				return due, err
			}
			due++
		}
	}
	if due > 0 {
		s.log.Info("scheduled sources")
	}
	return due, nil
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
