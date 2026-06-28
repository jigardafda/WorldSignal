package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

// closed returns a DB whose pool is closed, so every query errors — exercising
// the error-return branches across the data layer.
func closed(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Connect(context.Background(), dbtest.URL())
	if err != nil {
		t.Skip("no DB")
	}
	d.Close()
	return d
}

func TestDBErrorPaths(t *testing.T) {
	d := closed(t)
	ctx := context.Background()
	now := time.Now()
	url := "https://x.example"

	mustErr := func(name string, err error) {
		if err == nil {
			t.Fatalf("%s: expected error on closed pool", name)
		}
	}

	_, err := d.ListSources(ctx)
	mustErr("ListSources", err)
	_, err = d.GetSource(ctx, "x")
	mustErr("GetSource", err)
	_, err = d.GetStats(ctx)
	mustErr("GetStats", err)
	_, err = d.ListSignals(ctx, db.SignalFilter{})
	mustErr("ListSignals", err)
	_, err = d.GetSignal(ctx, "x")
	mustErr("GetSignal", err)
	_, err = d.ListSubscriptions(ctx)
	mustErr("ListSubscriptions", err)
	_, err = d.ListSubscriptionsBasic(ctx)
	mustErr("ListSubscriptionsBasic", err)
	_, err = d.ListDeliveries(ctx, 10)
	mustErr("ListDeliveries", err)
	_, err = d.CreateSource(ctx, db.CreateSourceInput{Name: "n", URL: url})
	mustErr("CreateSource", err)
	_, err = d.SetSourceEnabled(ctx, "x", true)
	mustErr("SetSourceEnabled", err)
	_, err = d.UpdateSource(ctx, "x", db.SourcePatch{})
	mustErr("UpdateSource", err)
	mustErr("UpsertDefaultSubscriber", d.UpsertDefaultSubscriber(ctx))
	_, err = d.CreateSubscription(ctx, db.CreateSubscriptionInput{Name: "n"})
	mustErr("CreateSubscription", err)
	_, err = d.GetRawItem(ctx, "x")
	mustErr("GetRawItem", err)
	mustErr("SetRawItemStatus", d.SetRawItemStatus(ctx, "x", "PARSED"))
	_, err = d.ArticleIDByRawItem(ctx, "x")
	mustErr("ArticleIDByRawItem", err)
	_, err = d.FindDuplicateArticle(ctx, "h", &url)
	mustErr("FindDuplicateArticle", err)
	_, err = d.FindDuplicateArticle(ctx, "h", nil)
	mustErr("FindDuplicateArticle nil url", err)
	_, err = d.CreateArticle(ctx, db.NewArticle{})
	mustErr("CreateArticle", err)
	_, err = d.GetClusterArticle(ctx, "x")
	mustErr("GetClusterArticle", err)
	_, err = d.ExistingSignalForArticle(ctx, "x")
	mustErr("ExistingSignalForArticle", err)
	_, err = d.RecentSignalCandidates(ctx, now)
	mustErr("RecentSignalCandidates", err)
	mustErr("AttachArticleToSignal", d.AttachArticleToSignal(ctx, "s", "a", 0.5, now))
	_, err = d.CreateSignalFromArticle(ctx, &db.ClusterArticle{ID: "a"}, now)
	mustErr("CreateSignalFromArticle", err)
	_, err = d.LoadSignalForEnrich(ctx, "x")
	mustErr("LoadSignalForEnrich", err)
	_, err = d.TagIDsByCodes(ctx, []string{"A"})
	mustErr("TagIDsByCodes", err)
	mustErr("ApplyEnrichment", d.ApplyEnrichment(ctx, "x", db.EnrichmentUpdate{PublishedAt: now, Metadata: map[string]any{}}))
	_, err = d.LoadSignalForMatch(ctx, "x")
	mustErr("LoadSignalForMatch", err)
	_, err = d.EnabledSubscriptions(ctx)
	mustErr("EnabledSubscriptions", err)
	_, err = d.CreateDeliveryIfNew(ctx, "s", "sg", "POLLING", []byte("{}"))
	mustErr("CreateDeliveryIfNew", err)
	_, err = d.LoadDeliveryForSend(ctx, "x")
	mustErr("LoadDeliveryForSend", err)
	mustErr("IncrementDeliveryAttempts", d.IncrementDeliveryAttempts(ctx, "x"))
	mustErr("MarkDeliverySent", d.MarkDeliverySent(ctx, "x", now))
	mustErr("MarkDeliveryFailed", d.MarkDeliveryFailed(ctx, "x", "FAILED", now, "m"))
	_, err = d.GetSourceForFetch(ctx, "x")
	mustErr("GetSourceForFetch", err)
	_, err = d.RawItemExists(ctx, "s", "g")
	mustErr("RawItemExists", err)
	_, err = d.CreateRawItem(ctx, db.NewRawItem{SourceID: "s"})
	mustErr("CreateRawItem", err)
	mustErr("MarkSourceFetchSuccess", d.MarkSourceFetchSuccess(ctx, "x", now))
	mustErr("MarkSourceFetchFailure", d.MarkSourceFetchFailure(ctx, "x", now))
}

func TestAuthDBErrorPaths(t *testing.T) {
	d := closed(t)
	ctx := context.Background()
	now := time.Now()
	mustErr := func(name string, err error) {
		if err == nil {
			t.Fatalf("%s: expected error on closed pool", name)
		}
	}
	_, err := d.CountUsers(ctx)
	mustErr("CountUsers", err)
	_, err = d.CreateUser(ctx, "e@x", "n", "h", "VIEWER")
	mustErr("CreateUser", err)
	_, err = d.GetUserByEmail(ctx, "e@x")
	mustErr("GetUserByEmail", err)
	_, err = d.GetUserByID(ctx, "id")
	mustErr("GetUserByID", err)
	_, err = d.ListUsers(ctx)
	mustErr("ListUsers", err)
	_, err = d.UpdateUser(ctx, "id", db.UserPatch{})
	mustErr("UpdateUser", err)
	mustErr("UpdatePassword", d.UpdatePassword(ctx, "id", "h"))
	_, err = d.DeleteUser(ctx, "id")
	mustErr("DeleteUser", err)
	mustErr("CreateSession", d.CreateSession(ctx, "u", "t", now))
	_, err = d.UserForToken(ctx, "t")
	mustErr("UserForToken", err)
	mustErr("DeleteSession", d.DeleteSession(ctx, "t"))
	_, err = d.CreateTeam(ctx, "n")
	mustErr("CreateTeam", err)
	_, err = d.ListTeams(ctx)
	mustErr("ListTeams", err)
	_, err = d.GetTeam(ctx, "id")
	mustErr("GetTeam", err)
	_, err = d.DeleteTeam(ctx, "id")
	mustErr("DeleteTeam", err)
	mustErr("AddTeamMember", d.AddTeamMember(ctx, "t", "u", "MEMBER"))
	_, err = d.RemoveTeamMember(ctx, "t", "u")
	mustErr("RemoveTeamMember", err)
	_, err = d.ListTeamMembers(ctx, "t")
	mustErr("ListTeamMembers", err)
}

func TestUserTeamHappyEdges(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	// nil lookups.
	if u, err := d.GetUserByEmail(ctx, "missing@x"); err != nil || u != nil {
		t.Fatalf("missing email: %v %v", u, err)
	}
	if u, err := d.GetUserByID(ctx, "missing"); err != nil || u != nil {
		t.Fatalf("missing id: %v %v", u, err)
	}
	if u, err := d.UserForToken(ctx, "missing"); err != nil || u != nil {
		t.Fatalf("missing token: %v %v", u, err)
	}
	if u, err := d.UpdateUser(ctx, "missing", db.UserPatch{}); err != nil || u != nil {
		t.Fatalf("update missing: %v %v", u, err)
	}
	if ok, err := d.DeleteUser(ctx, "missing"); err != nil || ok {
		t.Fatalf("delete missing: %v %v", ok, err)
	}
	if tm, err := d.GetTeam(ctx, "missing"); err != nil || tm != nil {
		t.Fatalf("team missing: %v %v", tm, err)
	}
	if ok, err := d.DeleteTeam(ctx, "missing"); err != nil || ok {
		t.Fatalf("delete team missing: %v %v", ok, err)
	}
	if ok, err := d.RemoveTeamMember(ctx, "t", "u"); err != nil || ok {
		t.Fatalf("remove member missing: %v %v", ok, err)
	}
	// duplicate email.
	if _, err := d.CreateUser(ctx, "dup@x", "n", "h", "VIEWER"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateUser(ctx, "dup@x", "n", "h", "VIEWER"); err != db.ErrDuplicateEmail {
		t.Fatalf("dup email want ErrDuplicateEmail, got %v", err)
	}
	// teams + members happy path + AddTeamMember upsert role.
	tm, err := d.CreateTeam(ctx, "T")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := d.CreateUser(ctx, "tm@x", "n", "h", "VIEWER")
	if err := d.AddTeamMember(ctx, tm.ID, u.ID, "MEMBER"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddTeamMember(ctx, tm.ID, u.ID, "OWNER"); err != nil { // upsert
		t.Fatal(err)
	}
	members, err := d.ListTeamMembers(ctx, tm.ID)
	if err != nil || len(members) != 1 || members[0].Role != "OWNER" {
		t.Fatalf("members: %+v %v", members, err)
	}
	teams, err := d.ListTeams(ctx)
	if err != nil || len(teams) != 1 || teams[0].MemberCount != 1 {
		t.Fatalf("teams: %+v %v", teams, err)
	}
	if u2, err := d.UpdateUser(ctx, u.ID, db.UserPatch{Name: strPtr("Renamed")}); err != nil || u2.Name != "Renamed" {
		t.Fatalf("update name: %v %v", u2, err)
	}
}

func strPtr(s string) *string { return &s }

func TestTagIDsByCodesEmpty(t *testing.T) {
	d := dbtest.Connect(t)
	m, err := d.TagIDsByCodes(context.Background(), nil)
	if err != nil || len(m) != 0 {
		t.Fatalf("empty codes: %v %v", m, err)
	}
}

func TestGetClusterArticleNullTokenSet(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`); err != nil {
		t.Fatal(err)
	}
	// tokenSet NULL → deref's nil branch.
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Article" ("id","sourceId","title") VALUES ('a','s','T')`); err != nil {
		t.Fatal(err)
	}
	a, err := d.GetClusterArticle(ctx, "a")
	if err != nil || a == nil || a.TokenSet != "" {
		t.Fatalf("null tokenSet: %+v %v", a, err)
	}
}

func TestCreateDeliveryConflict(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Subscriber" ("id","name","createdAt") VALUES ('__default__','D',now())`)
	ex(`INSERT INTO "Subscription" ("id","subscriberId","name","channel","filter","config","createdAt") VALUES ('sub','__default__','S','POLLING','{}','{}',now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','T','S',now(),now(),now())`)

	id1, err := d.CreateDeliveryIfNew(ctx, "sub", "sg", "POLLING", []byte(`{}`))
	if err != nil || id1 == "" {
		t.Fatalf("first create: %q %v", id1, err)
	}
	id2, err := d.CreateDeliveryIfNew(ctx, "sub", "sg", "POLLING", []byte(`{}`))
	if err != nil || id2 != "" {
		t.Fatalf("duplicate should be skipped: %q %v", id2, err)
	}
}

func TestApplyEnrichmentMetaMarshalError(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	err := d.ApplyEnrichment(context.Background(), "sg", db.EnrichmentUpdate{
		PublishedAt: time.Now(),
		Metadata:    map[string]any{"bad": make(chan int)}, // unmarshalable
	})
	if err == nil {
		t.Fatal("expected metadata marshal error")
	}
}

func TestCreateRawItemConflict(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `INSERT INTO "Source" ("id","name","url","updatedAt") VALUES ('s','S','https://s.example',now())`); err != nil {
		t.Fatal(err)
	}
	g := "guid-1"
	in := db.NewRawItem{SourceID: "s", SourceGuid: &g, RawTitle: "T", RawContent: "B"}
	id1, err := d.CreateRawItem(ctx, in)
	if err != nil || id1 == "" {
		t.Fatalf("first insert: %q %v", id1, err)
	}
	id2, err := d.CreateRawItem(ctx, in) // same (sourceId, sourceGuid)
	if err != nil || id2 != "" {
		t.Fatalf("duplicate raw item should be skipped: %q %v", id2, err)
	}
}

func TestListSignalsAllFilters(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	dbtest.SeedTaxonomy(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Signal" ("id","title","summary","status","severity","confidence","country","sourceCount","firstSeenAt","lastSeenAt","updatedAt") VALUES ('sg','Quake hits','A quake.','CONFIRMED','HIGH',0.8,'US',1,'2026-01-02T00:00:00Z','2026-01-02T00:00:00Z',now())`)
	ex(`INSERT INTO "SignalTag" ("signalId","tagId","confidence") SELECT 'sg',"id",0.9 FROM "TaxonomyTag" WHERE "code"='DISASTER.EARTHQUAKE'`)

	country, status, search := "US", "CONFIRMED", "quake"
	minConf := 0.5
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	out, err := d.ListSignals(ctx, db.SignalFilter{
		Country: &country, Status: &status, MinConfidence: &minConf,
		Since: &since, Search: &search, Tags: []string{"DISASTER.EARTHQUAKE"},
		Limit: 500, Offset: 0, // Limit>200 exercises the cap
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != "sg" {
		t.Fatalf("expected the matching signal, got %d rows", len(out))
	}
	if len(out[0].Tags) != 1 {
		t.Fatalf("aggregate tags wrong: %d", len(out[0].Tags))
	}
}

func TestArticleIDByRawItemMissing(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	id, err := d.ArticleIDByRawItem(context.Background(), "nope")
	if err != nil || id != "" {
		t.Fatalf("missing raw → empty: %q %v", id, err)
	}
}

func TestConnectBadURL(t *testing.T) {
	if _, err := db.Connect(context.Background(), "postg://bad::url"); err == nil {
		t.Fatal("expected connect error for bad URL")
	}
}

func TestConnectPingFailure(t *testing.T) {
	// Valid DSN syntax but an unreachable port → pool opens lazily, Ping fails.
	if _, err := db.Connect(context.Background(), "postgres://u:p@127.0.0.1:1/db?connect_timeout=1"); err == nil {
		t.Fatal("expected ping failure for unreachable server")
	}
}
