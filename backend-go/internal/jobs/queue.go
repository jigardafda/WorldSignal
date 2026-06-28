// Package jobs is a small Postgres-backed job queue replacing pg-boss: send with
// optional singleton dedupe, worker poll loops, retry with backoff, and
// dead-lettering after the retry limit.
package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/jsonx"
	"github.com/worldsignal/backend/internal/logging"
)

// schema is created on demand; lives alongside the application tables.
const schema = `
CREATE TABLE IF NOT EXISTS ws_jobs (
  id            text PRIMARY KEY,
  queue         text NOT NULL,
  data          jsonb NOT NULL DEFAULT '{}',
  state         text NOT NULL DEFAULT 'created',
  retry_count   int NOT NULL DEFAULT 0,
  retry_limit   int NOT NULL DEFAULT 0,
  retry_delay   int NOT NULL DEFAULT 0,
  retry_backoff boolean NOT NULL DEFAULT false,
  singleton_key text,
  start_after   timestamptz NOT NULL DEFAULT now(),
  created_at    timestamptz NOT NULL DEFAULT now(),
  started_at    timestamptz,
  completed_at  timestamptz,
  last_error    text
);
CREATE INDEX IF NOT EXISTS ws_jobs_poll ON ws_jobs(queue, state, start_after);`

// Handler processes a job's data. isFinalAttempt is true when the retry limit has
// been reached (used for dead-lettering decisions). Returning an error triggers a
// retry (or dead-letter on the final attempt).
type Handler func(ctx context.Context, data []byte, isFinalAttempt bool) error

// SendOptions configure a send.
type SendOptions struct {
	SingletonKey string
	RetryLimit   int
	RetryDelay   int // seconds
	RetryBackoff bool
}

// Queue is the job queue engine.
type Queue struct {
	pool      *pgxpool.Pool
	log       *logging.Logger
	handlers  map[string]Handler
	pollEvery time.Duration
	cancel    context.CancelFunc
	done      chan struct{}
}

// New creates a queue over the given pool.
func New(pool *pgxpool.Pool) *Queue {
	return &Queue{
		pool:      pool,
		log:       logging.New("jobs"),
		handlers:  map[string]Handler{},
		pollEvery: 200 * time.Millisecond,
	}
}

// Migrate ensures the jobs table exists.
func (q *Queue) Migrate(ctx context.Context) error {
	_, err := q.pool.Exec(ctx, schema)
	return err
}

// Send enqueues a job. With a SingletonKey, it is a no-op if an unfinished job
// with that key already exists in the queue.
func (q *Queue) Send(ctx context.Context, queue string, data any, opts SendOptions) error {
	payload, err := jsonx.Marshal(data)
	if err != nil {
		return err
	}
	id := cuid.New()
	if opts.SingletonKey != "" {
		_, err = q.pool.Exec(ctx,
			`INSERT INTO ws_jobs (id,queue,data,retry_limit,retry_delay,retry_backoff,singleton_key)
			 SELECT $1,$2,$3,$4,$5,$6,$7
			 WHERE NOT EXISTS (
			   SELECT 1 FROM ws_jobs WHERE queue=$2 AND singleton_key=$7 AND state IN ('created','active'))`,
			id, queue, payload, opts.RetryLimit, opts.RetryDelay, opts.RetryBackoff, opts.SingletonKey)
		return err
	}
	_, err = q.pool.Exec(ctx,
		`INSERT INTO ws_jobs (id,queue,data,retry_limit,retry_delay,retry_backoff)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		id, queue, payload, opts.RetryLimit, opts.RetryDelay, opts.RetryBackoff)
	return err
}

// RegisterWorker attaches a handler for a queue.
func (q *Queue) RegisterWorker(queue string, h Handler) {
	q.handlers[queue] = h
}

type claimedJob struct {
	id         string
	data       []byte
	retryCount int
	retryLimit int
	retryDelay int
	backoff    bool
}

// processOne claims and runs one job for a queue. Returns false if none ready.
func (q *Queue) processOne(ctx context.Context, queue string) (bool, error) {
	var j claimedJob
	err := q.pool.QueryRow(ctx,
		`UPDATE ws_jobs SET state='active', started_at=now()
		 WHERE id = (
		   SELECT id FROM ws_jobs WHERE queue=$1 AND state='created' AND start_after<=now()
		   ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1)
		 RETURNING id, data, retry_count, retry_limit, retry_delay, retry_backoff`, queue).
		Scan(&j.id, &j.data, &j.retryCount, &j.retryLimit, &j.retryDelay, &j.backoff)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, err
	}

	handler := q.handlers[queue]
	isFinal := j.retryCount >= j.retryLimit
	herr := handler(ctx, j.data, isFinal)
	if herr == nil {
		_, err = q.pool.Exec(ctx, `UPDATE ws_jobs SET state='completed', completed_at=now() WHERE id=$1`, j.id)
		return true, err
	}

	if j.retryCount < j.retryLimit {
		delay := j.retryDelay
		if j.backoff {
			delay = j.retryDelay * (1 << j.retryCount)
		}
		_, err = q.pool.Exec(ctx,
			`UPDATE ws_jobs SET state='created', retry_count=retry_count+1,
			 start_after=now() + ($2 || ' seconds')::interval, last_error=$3 WHERE id=$1`,
			j.id, itoa(delay), herr.Error())
		return true, err
	}
	// Exhausted retries → dead-letter.
	_, err = q.pool.Exec(ctx, `UPDATE ws_jobs SET state='failed', last_error=$2 WHERE id=$1`, j.id, herr.Error())
	return true, err
}

// ProcessOneForTest claims and runs a single job synchronously. Exported for
// deterministic testing of the queue mechanics.
func (q *Queue) ProcessOneForTest(ctx context.Context, queue string) (bool, error) {
	return q.processOne(ctx, queue)
}

// Start launches background poll loops for all registered workers.
func (q *Queue) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	q.cancel = cancel
	q.done = make(chan struct{})
	go func() {
		defer close(q.done)
		ticker := time.NewTicker(q.pollEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for queue := range q.handlers {
					for {
						worked, err := q.processOne(ctx, queue)
						if err != nil {
							q.log.Error("process failed", err.Error())
							break
						}
						if !worked {
							break
						}
					}
				}
			}
		}
	}()
}

// Stop halts the poll loops.
func (q *Queue) Stop() {
	if q.cancel != nil {
		q.cancel()
		<-q.done
		q.cancel = nil
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
