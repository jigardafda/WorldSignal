package parity_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/parity"
	"github.com/worldsignal/backend/internal/pipeline"
)

const deliverySnap = `SELECT row_to_json(t) FROM (SELECT "subscriptionId","signalId","channel","status","payload","attempts","errorMessage" FROM "DeliveryEvent" ORDER BY "subscriptionId") t`

func seedMatchInput(t *testing.T, d *db.DB) {
	t.Helper()
	ex := mkExec(t, d)
	ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','Default Subscriber',now())`)
	subs := []struct{ id, channel, filter string }{
		{"sub_all", "POLLING", `{}`},
		{"sub_disaster", "WEBHOOK", `{"tags":["DISASTER"]}`},
		{"sub_econ", "WEBHOOK", `{"tags":["ECONOMY"]}`},
		{"sub_highconf", "WEBHOOK", `{"minConfidence":0.9}`},
		{"sub_country", "WEBHOOK", `{"countries":["US"]}`},
		{"sub_sev", "WEBHOOK", `{"minSeverity":"HIGH"}`},
		{"sub_lowsev", "WEBHOOK", `{"minSeverity":"LOW"}`},
	}
	for _, s := range subs {
		ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ($1,'__default__',$1,$2::"DeliveryChannel",$3,'{}',now())`,
			s.id, s.channel, s.filter)
	}
	ex(`INSERT INTO "Signal" ("id","title","summary","status","severity","confidence","country","sourceCount","firstSeenAt","lastSeenAt","updatedAt")
		VALUES ('sig_m','Cyclone hits coast','Severe storm.','DEVELOPING','CRITICAL',0.71,'IN',2,'2026-01-02T00:00:00.000Z','2026-01-02T00:30:00.000Z',now())`)
	ex(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") SELECT 'sig_m', "id", 0.8 FROM "TaxonomyTag" WHERE "code"='DISASTER.CYCLONE'`)
}

func TestPipelineMatchParity(t *testing.T) {
	if testing.Short() {
		t.Skip("needs DB + node")
	}
	d := dbtest.Connect(t)
	ctx := context.Background()

	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	seedMatchInput(t, d)
	if _, err := parity.RunTSStage("match", `{"signalId":"sig_m"}`, dbtest.URL(), paritySecret); err != nil {
		t.Fatal(err)
	}
	tsDel := snapshot(t, d, deliverySnap)

	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	seedMatchInput(t, d)
	if _, err := pipeline.MatchSubscriptions(ctx, d, "sig_m", time.Now()); err != nil {
		t.Fatal(err)
	}
	goDel := snapshot(t, d, deliverySnap)

	eqSnapshots(t, "match deliveries", tsDel, goDel)
}
