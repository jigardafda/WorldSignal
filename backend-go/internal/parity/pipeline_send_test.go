package parity_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/parity"
	"github.com/worldsignal/backend/internal/pipeline"
)

type capturedReq struct {
	signature string
	eventID   string
	attempt   string
	body      string
}

type webhookStub struct {
	mu     sync.Mutex
	status int
	last   *capturedReq
}

func (s *webhookStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.last = &capturedReq{
		signature: r.Header.Get("X-WorldSignal-Signature"),
		eventID:   r.Header.Get("X-WorldSignal-Event-Id"),
		attempt:   r.Header.Get("X-WorldSignal-Attempt"),
		body:      string(body),
	}
	code := s.status
	s.mu.Unlock()
	if code == 0 {
		code = 200
	}
	w.WriteHeader(code)
}

func (s *webhookStub) take() *capturedReq {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.last
	s.last = nil
	return r
}

const envelopePayload = `{"schema_version":"2026-06-01","event_type":"signal.published","event_id":"evt_sig_s_sub_s","created_at":"2026-01-02T00:00:00.000Z","subscription_id":"sub_s","data":{"signal_id":"sig_s","title":"T","summary":"S","status":"CONFIRMED","severity":"HIGH","confidence":0.82,"country":"US","tags":["DISASTER.EARTHQUAKE"],"source_count":3,"first_seen_at":"2026-01-02T00:00:00.000Z","last_seen_at":"2026-01-02T00:30:00.000Z"}}`

func seedDelivery(t *testing.T, d *db.DB, channel, config string) {
	t.Helper()
	ex := mkExec(t, d)
	ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','Default Subscriber',now())`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub_s','__default__','S',$1::"DeliveryChannel",'{}',$2,now())`, channel, config)
	ex(`INSERT INTO "Signal" ("id","title","summary","status","severity","confidence","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sig_s','T','S','CONFIRMED','HIGH',0.82,3,'2026-01-02T00:00:00.000Z','2026-01-02T00:30:00.000Z',now())`)
	ex(`INSERT INTO "DeliveryEvent" ("id","subscriptionId","signalId","channel","status","payload","attempts","createdAt") VALUES ('del_s','sub_s','sig_s',$1::"DeliveryChannel",'PENDING',$2::jsonb,0,now())`, channel, envelopePayload)
}

func deliveryStatus(t *testing.T, d *db.DB) (string, *string) {
	t.Helper()
	var status string
	var msg *string
	if err := d.Pool.QueryRow(context.Background(), `SELECT "status","errorMessage" FROM "DeliveryEvent" WHERE "id"='del_s'`).Scan(&status, &msg); err != nil {
		t.Fatal(err)
	}
	return status, msg
}

func TestPipelineSendParity(t *testing.T) {
	if testing.Short() {
		t.Skip("needs DB + node")
	}
	d := dbtest.Connect(t)
	ctx := context.Background()
	stub := &webhookStub{status: 200}
	srv := httptest.NewServer(stub)
	defer srv.Close()
	hookConfig := `{"url":"` + srv.URL + `"}`

	type sc struct {
		name     string
		channel  string
		config   string
		stubCode int
		isFinal  bool
		wantHTTP bool
	}
	cases := []sc{
		{"webhook_success", "WEBHOOK", hookConfig, 200, false, true},
		{"polling", "POLLING", `{}`, 200, false, false},
		{"webhook_no_url", "WEBHOOK", `{}`, 200, false, false},
		{"webhook_fail_retry", "WEBHOOK", hookConfig, 500, false, true},
		{"webhook_fail_final", "WEBHOOK", hookConfig, 500, true, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isFinal := "false"
			if c.isFinal {
				isFinal = "true"
			}

			// TS run
			stub.status = c.stubCode
			dbtest.Reset(t, d)
			seedDelivery(t, d, c.channel, c.config)
			_, _ = parity.RunTSStage("send", `{"deliveryId":"del_s","isFinal":`+isFinal+`}`, dbtest.URL(), paritySecret)
			tsStatus, tsMsg := deliveryStatus(t, d)
			tsReq := stub.take()

			// Go run
			stub.status = c.stubCode
			dbtest.Reset(t, d)
			seedDelivery(t, d, c.channel, c.config)
			_ = pipeline.SendDelivery(ctx, d, srv.Client(), paritySecret, "del_s", c.isFinal, time.Now())
			goStatus, goMsg := deliveryStatus(t, d)
			goReq := stub.take()

			if tsStatus != goStatus {
				t.Fatalf("%s: status TS=%s Go=%s", c.name, tsStatus, goStatus)
			}
			if derefS(tsMsg) != derefS(goMsg) {
				t.Fatalf("%s: errorMessage TS=%q Go=%q", c.name, derefS(tsMsg), derefS(goMsg))
			}
			if c.wantHTTP {
				if tsReq == nil || goReq == nil {
					t.Fatalf("%s: expected webhook hits TS=%v Go=%v", c.name, tsReq, goReq)
				}
				if tsReq.signature != goReq.signature {
					t.Fatalf("%s: HMAC signature mismatch\nTS: %s\nGo: %s", c.name, tsReq.signature, goReq.signature)
				}
				if tsReq.body != goReq.body {
					t.Fatalf("%s: body mismatch\nTS: %s\nGo: %s", c.name, tsReq.body, goReq.body)
				}
				if tsReq.eventID != goReq.eventID || tsReq.attempt != goReq.attempt {
					t.Fatalf("%s: header mismatch TS=%+v Go=%+v", c.name, tsReq, goReq)
				}
			}
		})
	}
}

func derefS(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
