package pipeline

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/crypto"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/email"
)

func TestParseEmailConfig(t *testing.T) {
	// `to` as a string.
	c := parseEmailConfig([]byte(`{"connectorId":"c1","to":"a@x.com, b@y.com","mode":"Digest","interval":"Hourly"}`))
	if c.ConnectorID != "c1" || c.Mode != "digest" || c.Interval != "hourly" {
		t.Fatalf("parsed: %+v", c)
	}
	if len(c.Recipients) != 2 {
		t.Fatalf("recipients: %v", c.Recipients)
	}
	// `to` as an array.
	c2 := parseEmailConfig([]byte(`{"to":["a@x.com","a@x.com"]}`))
	if len(c2.Recipients) != 1 {
		t.Fatalf("array recipients dedup: %v", c2.Recipients)
	}
	// `to` as an unexpected JSON type (number) → no recipients.
	if r := parseEmailConfig([]byte(`{"to":123}`)).Recipients; len(r) != 0 {
		t.Errorf("numeric to should yield no recipients: %v", r)
	}
	// Empty / invalid config.
	if isDigestConfig(nil) {
		t.Error("nil config should not be digest")
	}
	if DigestIntervalFromConfig(nil) != "daily" {
		t.Error("default interval should be daily")
	}
	if DigestIntervalFromConfig([]byte(`{"interval":"hourly"}`)) != "hourly" {
		t.Error("hourly interval")
	}
}

func TestRenderDeliveryEmail(t *testing.T) {
	instant := []byte(`{"event_type":"signal.published","data":{"signal_id":"s1","title":"Quake","summary":"big","severity":"HIGH","country":"US","tags":["disaster"],"source_count":2}}`)
	subj, text, html := renderDeliveryEmail(instant)
	if !strings.Contains(subj, "HIGH") || !strings.Contains(text, "Quake") || !strings.Contains(html, "Quake") {
		t.Fatalf("instant render: %q / %q", subj, text)
	}
	digest := []byte(`{"event_type":"signal.digest","data":{"interval":"daily","count":1,"signals":[{"signal_id":"s1","title":"Alpha","severity":"LOW","last_seen_at":"2026-06-01T00:00:00.000Z","link":"https://n/a"}]}}`)
	dsubj, dtext, _ := renderDeliveryEmail(digest)
	if !strings.Contains(dsubj, "digest") || !strings.Contains(dtext, "Alpha") {
		t.Fatalf("digest render: %q", dsubj)
	}
}

func TestRelTime(t *testing.T) {
	if relTime("") != "" {
		t.Error("empty in → empty out")
	}
	if relTime("not-a-date") != "" {
		t.Error("bad date → empty")
	}
	now := time.Now().UTC()
	if got := relTime(now.Add(-2 * time.Hour).Format(isoLayout)); !strings.HasSuffix(got, "h ago") {
		t.Errorf("hours: %q", got)
	}
	if got := relTime(now.Add(-3 * 24 * time.Hour).Format(isoLayout)); !strings.HasSuffix(got, "d ago") {
		t.Errorf("days: %q", got)
	}
	if got := relTime(now.Add(-30 * time.Second).Format(isoLayout)); got != "just now" {
		t.Errorf("just now: %q", got)
	}
	// RFC3339 without milliseconds parses via the fallback layout.
	if got := relTime(now.Add(-5 * time.Minute).Format(time.RFC3339)); !strings.HasSuffix(got, "m ago") {
		t.Errorf("rfc3339 fallback: %q", got)
	}
}

// stubEmail swaps the package send function for the duration of a test.
func stubEmail(t *testing.T, fn func(context.Context, email.SMTPConfig, email.Message) error) {
	t.Helper()
	prev := emailSend
	emailSend = fn
	t.Cleanup(func() { emailSend = prev })
}

func seedEmailConnector(t *testing.T, d *db.DB, secret string) {
	t.Helper()
	cipher, err := crypto.Encrypt(secret, "app-password")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, d, `INSERT INTO "EmailConnector" ("id","name","provider","host","port","security","username","secretCiphertext","secretLast4","fromEmail","fromName","isActive","enabled","status","updatedAt","createdAt") VALUES ('conn','C','GMAIL','smtp.gmail.com',587,'STARTTLS','me@gmail.com',$1,'word','me@gmail.com','WorldSignal',true,true,'VALID',now(),now())`, cipher)
}

func TestSendEmailDelivery(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	now := time.Now()
	dbtest.Reset(t, d)
	seedEmailConnector(t, d, "secret")
	mustExec(t, d, `INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	mustExec(t, d, `INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub','__default__','S','EMAIL','{}','{"to":"reader@example.com"}',now())`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S',now(),now(),now())`)
	mustExec(t, d, `INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","attempts","createdAt") VALUES ('del','sub','sg','EMAIL','PENDING','{"event_type":"signal.published","data":{"title":"T","severity":"LOW"}}',0,now())`)

	var gotMsg email.Message
	var gotCfg email.SMTPConfig
	stubEmail(t, func(_ context.Context, cfg email.SMTPConfig, m email.Message) error {
		gotCfg, gotMsg = cfg, m
		return nil
	})

	if err := SendDelivery(ctx, d, nil, "secret", "del", false, now); err != nil {
		t.Fatalf("send: %v", err)
	}
	var status string
	mustScan(t, d, `SELECT status FROM "DeliveryEvent" WHERE id='del'`, &status)
	if status != "SENT" {
		t.Fatalf("status %s", status)
	}
	if gotCfg.Host != "smtp.gmail.com" || gotCfg.Password != "app-password" {
		t.Fatalf("connector not resolved/decrypted: %+v", gotCfg)
	}
	if len(gotMsg.To) != 1 || gotMsg.To[0] != "reader@example.com" {
		t.Fatalf("recipients: %v", gotMsg.To)
	}
}

func TestSendEmailDeliveryNoRecipients(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	dbtest.Reset(t, d)
	seedEmailConnector(t, d, "secret")
	mustExec(t, d, `INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	mustExec(t, d, `INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub','__default__','S','EMAIL','{}','{}',now())`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S',now(),now(),now())`)
	mustExec(t, d, `INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","attempts","createdAt") VALUES ('del','sub','sg','EMAIL','PENDING','{}',0,now())`)

	// No recipients → non-final failure returns error and marks RETRYING.
	if err := SendDelivery(ctx, d, nil, "secret", "del", false, time.Now()); err == nil {
		t.Fatal("expected error for missing recipients")
	}
	var status string
	mustScan(t, d, `SELECT status FROM "DeliveryEvent" WHERE id='del'`, &status)
	if status != "RETRYING" {
		t.Fatalf("status %s", status)
	}
}

func TestSendEmailDeliveryDecryptError(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	dbtest.Reset(t, d)
	// Connector whose ciphertext can't be decrypted with the delivery secret.
	mustExec(t, d, `INSERT INTO "EmailConnector" ("id","name","provider","host","port","security","username","secretCiphertext","secretLast4","fromEmail","fromName","isActive","enabled","status","updatedAt","createdAt") VALUES ('conn','C','GMAIL','smtp.gmail.com',587,'STARTTLS','me@gmail.com','bad-cipher!!','xxxx','me@gmail.com','WS',true,true,'VALID',now(),now())`)
	mustExec(t, d, `INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	mustExec(t, d, `INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub','__default__','S','EMAIL','{}','{"to":"r@x.com"}',now())`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S',now(),now(),now())`)
	mustExec(t, d, `INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","attempts","createdAt") VALUES ('del','sub','sg','EMAIL','PENDING','{}',0,now())`)

	if err := SendDelivery(ctx, d, nil, "secret", "del", false, time.Now()); err == nil {
		t.Fatal("expected decrypt error")
	}
	var status string
	mustScan(t, d, `SELECT status FROM "DeliveryEvent" WHERE id='del'`, &status)
	if status != "RETRYING" {
		t.Fatalf("status %s", status)
	}
}

func TestSendEmailDeliveryNoConnector(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	dbtest.Reset(t, d)
	// No connector seeded at all.
	mustExec(t, d, `INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	mustExec(t, d, `INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub','__default__','S','EMAIL','{}','{"to":"r@x.com"}',now())`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S',now(),now(),now())`)
	mustExec(t, d, `INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","attempts","createdAt") VALUES ('del','sub','sg','EMAIL','PENDING','{}',0,now())`)

	if err := SendDelivery(ctx, d, nil, "secret", "del", true, time.Now()); err != nil {
		t.Fatalf("final attempt should swallow error: %v", err)
	}
	var status string
	mustScan(t, d, `SELECT status FROM "DeliveryEvent" WHERE id='del'`, &status)
	if status != "DEAD_LETTERED" {
		t.Fatalf("status %s", status)
	}
}

func TestMatchSubscriptionsDigestQueue(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	dbtest.SeedTaxonomy(t, d)
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","severity","confidence","country","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S','HIGH',0.8,'US',1,now(),now(),now())`)
	mustExec(t, d, `INSERT INTO "SignalTag" ("signalId","tagId","confidence") SELECT 'sg',"id",0.9 FROM "TaxonomyTag" WHERE "code"='DISASTER.EARTHQUAKE'`)
	mustExec(t, d, `INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	// Digest-mode email subscription → queued, not delivered immediately.
	mustExec(t, d, `INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('dig','__default__','digest','EMAIL','{"tags":["DISASTER"]}','{"mode":"digest","interval":"daily","to":"r@x.com"}',now())`)

	ids, err := MatchSubscriptions(ctx, d, "sg", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("digest subscription should not create an immediate delivery, got %d", len(ids))
	}
	var queued int
	mustScan(t, d, `SELECT count(*) FROM "DigestQueue" WHERE "subscriptionId"='dig'`, &queued)
	if queued != 1 {
		t.Fatalf("expected 1 queued digest item, got %d", queued)
	}
}

func TestBuildDigest(t *testing.T) {
	d := conn(t)
	ctx := context.Background()
	now := time.Now()
	dbtest.Reset(t, d)
	mustExec(t, d, `INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	mustExec(t, d, `INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('dig','__default__','digest','EMAIL','{}','{"mode":"digest","interval":"daily","to":"r@x.com"}',now())`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","severity","confidence","country","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('s1','First','A',	'HIGH',0.8,'IN',1,now(),now(),now())`)
	mustExec(t, d, `INSERT INTO "Signal" ("id","title","summary","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('s2','Second','B','LOW',0.5,1,now(),now() - interval '1 minute',now())`)
	mustExec(t, d, `INSERT INTO "DigestQueue" ("subscriptionId","signalId","queuedAt") VALUES ('dig','s1',now()),('dig','s2',now())`)

	deliveryID, count, err := BuildDigest(ctx, d, "dig", "daily", now)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || deliveryID == "" {
		t.Fatalf("count=%d id=%q", count, deliveryID)
	}
	// The queue is drained and lastDigestAt is stamped.
	var remaining int
	mustScan(t, d, `SELECT count(*) FROM "DigestQueue" WHERE "subscriptionId"='dig'`, &remaining)
	if remaining != 0 {
		t.Fatalf("queue not drained: %d", remaining)
	}
	// A single EMAIL delivery with a digest payload was created.
	var channel, payload string
	mustScan(t, d, `SELECT channel,payload FROM "DeliveryEvent" WHERE "subscriptionId"='dig'`, &channel, &payload)
	if channel != "EMAIL" {
		t.Fatalf("channel: %s", channel)
	}
	if !strings.Contains(payload, "signal.digest") || !strings.Contains(payload, "First") {
		t.Fatalf("payload: %s", payload)
	}

	// Building again with an empty queue yields nothing.
	id2, c2, err := BuildDigest(ctx, d, "dig", "daily", now)
	if err != nil || id2 != "" || c2 != 0 {
		t.Fatalf("empty digest: id=%q c=%d err=%v", id2, c2, err)
	}
}
