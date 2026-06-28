package jobs_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/jobs"
	"github.com/worldsignal/backend/internal/llm"
)

// TestWorkerHandlersUnmarshalError exercises each handler's data-unmarshal error
// branch by enqueuing a job whose payload has the wrong field type.
func TestWorkerHandlersUnmarshalError(t *testing.T) {
	d := dbtest.Connect(t)
	ctx := context.Background()
	dbtest.Reset(t, d)
	q := jobs.New(d.Pool)
	if err := q.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `TRUNCATE TABLE ws_jobs`); err != nil {
		t.Fatal(err)
	}
	w := jobs.NewWorkers(q, d, llm.NewOpenAIGateway("", ""), "secret")
	w.Register()

	queues := []string{jobs.QFetchSource, jobs.QProcessArticle, jobs.QEnrichSignal, jobs.QMatchSignal, jobs.QSendDelivery}
	for _, queue := range queues {
		// Number where a string id is expected → json.Unmarshal error in handler.
		if err := q.Send(ctx, queue, map[string]int{"sourceId": 1, "rawItemId": 1, "signalId": 1, "deliveryId": 1}, jobs.SendOptions{}); err != nil {
			t.Fatal(err)
		}
		worked, err := q.ProcessOneForTest(ctx, queue)
		if err != nil || !worked {
			t.Fatalf("%s: worked=%v err=%v", queue, worked, err)
		}
		var state string
		if err := d.Pool.QueryRow(ctx, `SELECT state FROM ws_jobs WHERE queue=$1`, queue).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state != "failed" { // retryLimit 0 → handler error dead-letters immediately
			t.Fatalf("%s: expected failed, got %s", queue, state)
		}
	}
}
