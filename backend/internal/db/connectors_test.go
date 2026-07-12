package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func newConn(t *testing.T, d *db.DB, name string) *db.EmailConnector {
	t.Helper()
	by := "admin"
	c, err := d.CreateEmailConnector(context.Background(), cuid.New(), db.CreateEmailConnectorInput{
		Name: name, Provider: "GMAIL", Host: "smtp.gmail.com", Port: 587, Security: "STARTTLS",
		Username: "me@gmail.com", Ciphertext: "ct", Last4: "word", FromEmail: "me@gmail.com", FromName: "WS",
		CreatedBy: &by,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	return c
}

func TestEmailConnectorStore(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()

	c1 := newConn(t, d, "First")
	if c1.IsActive || !c1.Enabled || c1.Status != "UNTESTED" {
		t.Fatalf("defaults: %+v", c1)
	}
	c2 := newConn(t, d, "Second")

	// No active connector yet.
	if a, _ := d.GetActiveEmailConnector(ctx); a != nil {
		t.Fatal("expected no active connector")
	}
	// Resolve with unknown id falls back to active (still nil here).
	if r, _ := d.ResolveEmailConnector(ctx, "nope"); r != nil {
		t.Fatal("expected nil resolve")
	}

	// Activate c2 → sole active.
	if _, err := d.SetActiveEmailConnector(ctx, c2.ID); err != nil {
		t.Fatal(err)
	}
	active, _ := d.GetActiveEmailConnector(ctx)
	if active == nil || active.ID != c2.ID {
		t.Fatalf("active: %+v", active)
	}
	// Activate c1 → c2 deactivated.
	if _, err := d.SetActiveEmailConnector(ctx, c1.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := d.ListEmailConnectors(ctx)
	if len(list) != 2 || !list[0].IsActive || list[0].ID != c1.ID {
		t.Fatalf("list order/active: %+v", list)
	}

	// Resolve: explicit id wins when enabled.
	r, _ := d.ResolveEmailConnector(ctx, c2.ID)
	if r == nil || r.ID != c2.ID {
		t.Fatalf("resolve explicit: %+v", r)
	}

	// Update: partial change marks untested; secret unchanged when not supplied.
	name := "Renamed"
	host := "smtp.new"
	port := 465
	sec := "TLS"
	dis := false
	upd, err := d.UpdateEmailConnector(ctx, c2.ID, db.UpdateEmailConnectorInput{
		Name: &name, Host: &host, Port: &port, Security: &sec, Enabled: &dis,
	})
	if err != nil {
		t.Fatal(err)
	}
	if upd.Name != "Renamed" || upd.Host != "smtp.new" || upd.Port != 465 || upd.Security != "TLS" || upd.Enabled {
		t.Fatalf("update: %+v", upd)
	}
	if upd.SecretCiphertext != "ct" {
		t.Fatalf("secret should be preserved, got %q", upd.SecretCiphertext)
	}
	// A disabled explicit connector falls back to the active one.
	r2, _ := d.ResolveEmailConnector(ctx, c2.ID)
	if r2 == nil || r2.ID != c1.ID {
		t.Fatalf("disabled explicit should fall back to active: %+v", r2)
	}
	// Update with a new secret replaces it.
	ct := "newct"
	l4 := "abcd"
	upd2, _ := d.UpdateEmailConnector(ctx, c2.ID, db.UpdateEmailConnectorInput{Ciphertext: &ct, Last4: &l4})
	if upd2.SecretCiphertext != "newct" || upd2.SecretLast4 != "abcd" {
		t.Fatalf("secret update: %+v", upd2)
	}
	// Update the from-address fields.
	fe, fn, un := "new@x.com", "New Name", "user2"
	upd3, _ := d.UpdateEmailConnector(ctx, c2.ID, db.UpdateEmailConnectorInput{FromEmail: &fe, FromName: &fn, Username: &un})
	if upd3.FromEmail != fe || upd3.FromName != fn || upd3.Username != un {
		t.Fatalf("from update: %+v", upd3)
	}

	// Status update.
	if err := d.UpdateEmailConnectorStatus(ctx, c1.ID, "VALID", nil); err != nil {
		t.Fatal(err)
	}
	got, _ := d.GetEmailConnector(ctx, c1.ID)
	if got.Status != "VALID" || got.LastTestedAt == nil {
		t.Fatalf("status: %+v", got)
	}

	// Missing rows return nil, nil.
	if g, err := d.GetEmailConnector(ctx, "missing"); g != nil || err != nil {
		t.Fatalf("missing get: %+v %v", g, err)
	}
	if u, err := d.UpdateEmailConnector(ctx, "missing", db.UpdateEmailConnectorInput{Name: &name}); u != nil || err != nil {
		t.Fatalf("missing update: %+v %v", u, err)
	}
	if a, err := d.SetActiveEmailConnector(ctx, "missing"); a != nil || err != nil {
		t.Fatalf("missing activate: %+v %v", a, err)
	}

	// Delete.
	ok, err := d.DeleteEmailConnector(ctx, c2.ID)
	if err != nil || !ok {
		t.Fatalf("delete: %v %v", ok, err)
	}
	if ok, _ := d.DeleteEmailConnector(ctx, c2.ID); ok {
		t.Fatal("double delete should report false")
	}
}

func TestConnectorErrorPaths(t *testing.T) {
	d := closed(t)
	ctx := context.Background()
	now := time.Now()
	s := "x"
	sp := &s
	mustErr := func(name string, err error) {
		if err == nil {
			t.Fatalf("%s: expected error on closed pool", name)
		}
	}
	_, err := d.ListEmailConnectors(ctx)
	mustErr("list", err)
	_, err = d.CreateEmailConnector(ctx, "id", db.CreateEmailConnectorInput{Name: "n", Host: "h", Port: 1, Security: "NONE", FromEmail: "a@x"})
	mustErr("create", err)
	_, err = d.UpdateEmailConnector(ctx, "id", db.UpdateEmailConnectorInput{Name: sp})
	mustErr("update", err)
	_, err = d.SetActiveEmailConnector(ctx, "id")
	mustErr("setActive", err)
	mustErr("status", d.UpdateEmailConnectorStatus(ctx, "id", "VALID", nil))
	_, err = d.DeleteEmailConnector(ctx, "id")
	mustErr("delete", err)
	mustErr("queue", d.QueueDigestItem(ctx, "s", "sg", now))
	_, err = d.PendingDigests(ctx)
	mustErr("pending", err)
	_, _, err = d.BuildDigestDelivery(ctx, "s", now, func([]db.DigestSignal) (string, []byte) { return "", nil })
	mustErr("build", err)
}

func TestBuildDigestDeliveryInnerErrors(t *testing.T) {
	d := dbtest.Connect(t)
	ctx := context.Background()
	now := time.Now()
	seed := func() {
		dbtest.Reset(t, d)
		ex := func(q string, a ...any) {
			if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
				t.Fatalf("exec: %v", err)
			}
		}
		ex(`INSERT INTO "Subscription" ("id","name","channel","filter","config","createdAt") VALUES ('dig','d','EMAIL','{}','{}',now())`)
		ex(`INSERT INTO "Signal" ("id","title","summary","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('s1','T','S','HIGH',0.8,1,now(),now(),now())`)
		ex(`INSERT INTO "DigestQueue" ("subscriptionId","signalId","queuedAt") VALUES ('dig','s1',now())`)
	}
	build := func([]db.DigestSignal) (string, []byte) { return "s1", []byte(`{"x":1}`) }
	hide := func(tbl string) func() {
		if _, err := d.Pool.Exec(ctx, `ALTER TABLE "`+tbl+`" RENAME TO "`+tbl+`__h"`); err != nil {
			t.Fatal(err)
		}
		return func() { _, _ = d.Pool.Exec(ctx, `ALTER TABLE "`+tbl+`__h" RENAME TO "`+tbl+`"`) }
	}

	// Main select fails when the Article table (used by the link subselect) is gone.
	seed()
	restore := hide("Article")
	if _, _, err := d.BuildDigestDelivery(ctx, "dig", now, build); err == nil {
		t.Fatal("expected main-select error")
	}
	restore()

	// Tag attach fails when SignalTag is gone (main select still works).
	seed()
	restore = hide("SignalTag")
	if _, _, err := d.BuildDigestDelivery(ctx, "dig", now, build); err == nil {
		t.Fatal("expected tag-attach error")
	}
	restore()

	// Delivery insert fails when DeliveryEvent is gone.
	seed()
	restore = hide("DeliveryEvent")
	if _, _, err := d.BuildDigestDelivery(ctx, "dig", now, build); err == nil {
		t.Fatal("expected insert error")
	}
	restore()
}

func TestDigestQueueAndBuild(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	now := time.Now()

	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
	ex(`INSERT INTO "Subscription" ("id","name","channel","filter","config","createdAt") VALUES ('dig','digest','EMAIL','{}','{"mode":"digest","interval":"hourly"}',now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('s1','First','A','HIGH',0.8,1,now(),now(),now())`)

	// Idempotent queueing.
	if err := d.QueueDigestItem(ctx, "dig", "s1", now); err != nil {
		t.Fatal(err)
	}
	if err := d.QueueDigestItem(ctx, "dig", "s1", now); err != nil {
		t.Fatal(err)
	}
	pend, err := d.PendingDigests(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pend) != 1 || pend[0].QueuedCount != 1 || pend[0].SubscriptionID != "dig" {
		t.Fatalf("pending: %+v", pend)
	}
	if pend[0].LastDigestAt != nil {
		t.Fatal("lastDigestAt should be nil initially")
	}

	// Build with a real payload.
	var built []db.DigestSignal
	id, count, err := d.BuildDigestDelivery(ctx, "dig", now, func(sigs []db.DigestSignal) (string, []byte) {
		built = sigs
		return sigs[0].ID, []byte(`{"event_type":"signal.digest"}`)
	})
	if err != nil || id == "" || count != 1 {
		t.Fatalf("build: id=%q count=%d err=%v", id, count, err)
	}
	if len(built) != 1 || built[0].Title != "First" || built[0].Severity != "HIGH" {
		t.Fatalf("built signals: %+v", built)
	}

	// Queue drained; lastDigestAt stamped.
	pend2, _ := d.PendingDigests(ctx)
	if len(pend2) != 0 {
		t.Fatalf("expected empty pending, got %+v", pend2)
	}
	var last *time.Time
	if err := d.Pool.QueryRow(ctx, `SELECT "lastDigestAt" FROM "Subscription" WHERE id='dig'`).Scan(&last); err != nil {
		t.Fatal(err)
	}
	if last == nil {
		t.Fatal("lastDigestAt not stamped")
	}

	// Build over an empty queue returns nothing and doesn't create a delivery.
	id2, count2, err := d.BuildDigestDelivery(ctx, "dig", now, func(sigs []db.DigestSignal) (string, []byte) {
		return "", nil
	})
	if err != nil || id2 != "" || count2 != 0 {
		t.Fatalf("empty build: id=%q count=%d err=%v", id2, count2, err)
	}
}
