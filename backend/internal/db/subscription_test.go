package db_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestSendTestDelivery(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	sub, err := d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "s", Channel: "SSE"})
	if err != nil {
		t.Fatal(err)
	}

	// No signals yet → returns "" (nothing to build a test event from).
	if id, err := d.SendTestDelivery(ctx, sub.ID); err != nil || id != "" {
		t.Fatalf("no-signal test delivery: id=%q err=%v", id, err)
	}

	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Signal"("id","title","summary","status","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES('s1','Quake','sum','CONFIRMED','HIGH',0.9,1,now(),now(),now())`); err != nil {
		t.Fatal(err)
	}
	id1, err := d.SendTestDelivery(ctx, sub.ID)
	if err != nil || id1 == "" {
		t.Fatalf("test delivery: id=%q err=%v", id1, err)
	}
	seq := func(id string) int64 {
		var s int64
		if err := d.Pool.QueryRow(ctx, `SELECT "seq" FROM "DeliveryEvent" WHERE "id"=$1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	seq1 := seq(id1)

	// A repeat reuses the same row but bumps seq so streaming cursors see it anew.
	id2, err := d.SendTestDelivery(ctx, sub.ID)
	if err != nil || id2 != id1 {
		t.Fatalf("repeat should reuse the row: id=%q err=%v", id2, err)
	}
	if seq(id2) <= seq1 {
		t.Fatalf("seq should bump on repeat: %d -> %d", seq1, seq(id2))
	}

	// The payload is flagged as a test event.
	var isTest bool
	if err := d.Pool.QueryRow(ctx, `SELECT (payload->>'test')::bool FROM "DeliveryEvent" WHERE "id"=$1`, id1).Scan(&isTest); err != nil || !isTest {
		t.Fatalf("payload should be marked test: %v %v", isTest, err)
	}

	// A pull channel (SSE) has no push step, so the row IS the delivery: SENT.
	var sseStatus string
	if err := d.Pool.QueryRow(ctx, `SELECT "status"::text FROM "DeliveryEvent" WHERE "id"=$1`, id1).Scan(&sseStatus); err != nil {
		t.Fatal(err)
	}
	if sseStatus != "SENT" {
		t.Fatalf("SSE test delivery should be SENT, got %q", sseStatus)
	}
}

// TestSendTestDeliveryWebhookPending guards the fix for the webhook test-delivery
// bug: a WEBHOOK test delivery must start PENDING so the enqueued worker job
// actually POSTs it to the configured URL. If it started SENT, SendDelivery's
// "already sent" short-circuit would silently skip the POST.
func TestSendTestDeliveryWebhookPending(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	sub, err := d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "w", Channel: "WEBHOOK"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Signal"("id","title","summary","status","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES('s1','Quake','sum','CONFIRMED','HIGH',0.9,1,now(),now(),now())`); err != nil {
		t.Fatal(err)
	}
	id, err := d.SendTestDelivery(ctx, sub.ID)
	if err != nil || id == "" {
		t.Fatalf("webhook test delivery: id=%q err=%v", id, err)
	}
	var status string
	if err := d.Pool.QueryRow(ctx, `SELECT "status"::text FROM "DeliveryEvent" WHERE "id"=$1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "PENDING" {
		t.Fatalf("webhook test delivery must be PENDING so the worker POSTs it, got %q", status)
	}

	// A repeat re-arms the row back to PENDING (with attempts reset) so re-testing
	// a subscription whose previous attempt was sent/failed still fires afresh.
	if _, err := d.Pool.Exec(ctx, `UPDATE "DeliveryEvent" SET "status"='SENT',"attempts"=3 WHERE "id"=$1`, id); err != nil {
		t.Fatal(err)
	}
	id2, err := d.SendTestDelivery(ctx, sub.ID)
	if err != nil || id2 != id {
		t.Fatalf("repeat should reuse the row: id=%q err=%v", id2, err)
	}
	var status2 string
	var attempts int
	if err := d.Pool.QueryRow(ctx, `SELECT "status"::text,"attempts" FROM "DeliveryEvent" WHERE "id"=$1`, id).Scan(&status2, &attempts); err != nil {
		t.Fatal(err)
	}
	if status2 != "PENDING" || attempts != 0 {
		t.Fatalf("repeat should re-arm to PENDING with attempts=0, got status=%q attempts=%d", status2, attempts)
	}
}
