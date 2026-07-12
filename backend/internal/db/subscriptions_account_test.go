package db_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestSubscriptionAccountScoping(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()

	acc, _ := d.CreateAccount(ctx, cuid.New(), "Acme", "acme", "PRO")
	sub, err := d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "s", AccountID: acc.ID})
	if err != nil {
		t.Fatalf("create sub: %v", err)
	}
	if sub.AccountID != acc.ID {
		t.Fatalf("sub account = %q want %q", sub.AccountID, acc.ID)
	}
	// A create with no account defaults to the default account.
	def, _ := d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "d"})
	if def.AccountID != db.DefaultAccountID {
		t.Fatalf("default sub account = %q", def.AccountID)
	}

	// SubscriptionAccountID: found + not found.
	if aid, _ := d.SubscriptionAccountID(ctx, sub.ID); aid != acc.ID {
		t.Fatalf("SubscriptionAccountID = %q", aid)
	}
	if aid, err := d.SubscriptionAccountID(ctx, "missing"); err != nil || aid != "" {
		t.Fatalf("missing sub should be (\"\",nil): %q %v", aid, err)
	}

	// Account-scoped list includes only the account's subscriptions with includes.
	subs, err := d.ListSubscriptionsByAccount(ctx, acc.ID)
	if err != nil || len(subs) != 1 || subs[0].Account == nil {
		t.Fatalf("ListSubscriptionsByAccount: %+v %v", subs, err)
	}

	// Deliveries scoped by account.
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Signal" ("id","title","summary","status","severity","confidence","country","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S','CONFIRMED','HIGH',0.8,'US',1,now(),now(),now())`)
	ex(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","createdAt") VALUES ('d1',$1,'sg','WEBHOOK','SENT','{}',now())`, sub.ID)
	dels, err := d.ListDeliveriesByAccount(ctx, acc.ID, 0)
	if err != nil || len(dels) != 1 || dels[0].SignalTitle != "T" {
		t.Fatalf("ListDeliveriesByAccount: %+v %v", dels, err)
	}
}

func TestSubscriptionAccountScopingDBErrors(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	acc, _ := d.CreateAccount(ctx, cuid.New(), "Acme", "acme", "PRO")
	if _, err := d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "s", AccountID: acc.ID}); err != nil {
		t.Fatal(err)
	}

	// DeliveryEvent gone → count include (in ListSubscriptionsByAccount) and
	// ListDeliveriesByAccount error.
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "DeliveryEvent" RENAME TO "DeliveryEvent__h"`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ListSubscriptionsByAccount(ctx, acc.ID); err == nil {
		t.Fatal("ListSubscriptionsByAccount should error (count include)")
	}
	if _, err := d.ListDeliveriesByAccount(ctx, acc.ID, 10); err == nil {
		t.Fatal("ListDeliveriesByAccount should error")
	}
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "DeliveryEvent__h" RENAME TO "DeliveryEvent"`); err != nil {
		t.Fatal(err)
	}

	// Account gone → owning-account include error.
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Account" RENAME TO "Account__h"`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ListSubscriptionsByAccount(ctx, acc.ID); err == nil {
		t.Fatal("ListSubscriptionsByAccount should error (account include)")
	}
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Account__h" RENAME TO "Account"`); err != nil {
		t.Fatal(err)
	}

	// Subscription gone → list + accountId lookup error.
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "Subscription" RENAME TO "Subscription__h"`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.Pool.Exec(ctx, `ALTER TABLE "Subscription__h" RENAME TO "Subscription"`) }()
	if _, err := d.ListSubscriptionsBasicByAccount(ctx, acc.ID); err == nil {
		t.Fatal("ListSubscriptionsBasicByAccount should error")
	}
	if _, err := d.SubscriptionAccountID(ctx, "x"); err == nil {
		t.Fatal("SubscriptionAccountID should error")
	}
	if _, err := d.ListDeliveriesByAccount(ctx, acc.ID, 10); err == nil {
		t.Fatal("ListDeliveriesByAccount should error (join)")
	}
}
