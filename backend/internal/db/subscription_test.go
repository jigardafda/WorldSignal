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

	// An explicit subscriber id is honoured.
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
