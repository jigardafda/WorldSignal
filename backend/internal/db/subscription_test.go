package db_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestCreateSubscriptionSubscriber(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Subscriber"("id","name","createdAt") VALUES('acme','Acme',now())`); err != nil {
		t.Fatal(err)
	}

	// An explicit subscriber id is honored.
	s, err := d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "stream", Channel: "SSE", SubscriberID: "acme"})
	if err != nil {
		t.Fatal(err)
	}
	if s.SubscriberID != "acme" || s.Channel != "SSE" {
		t.Fatalf("got subscriber=%q channel=%q", s.SubscriberID, s.Channel)
	}

	// Omitting it falls back to the auto-provisioned default subscriber.
	def, err := d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "d"})
	if err != nil {
		t.Fatal(err)
	}
	if def.SubscriberID != "__default__" {
		t.Fatalf("default subscriber = %q", def.SubscriberID)
	}
}

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
}
