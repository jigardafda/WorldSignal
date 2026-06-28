package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestSourceCountCoverage(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	seedEntities(t, d)
	// Enrich src1 with metadata so coverage/filter have something to group.
	d.Pool.Exec(context.Background(), `UPDATE "Source" SET "region"='North America',"geographicScope"='NATIONAL',"languages"='{en}',"industry"='Technology',"validationStatus"='VALID',"healthScore"=95,"sourceType"='RSS' WHERE "id"='src1'`)
	base := newServerWith(t, d, &recordEnqueuer{})
	tok, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)

	if b := gql(t, base, tok, `{"query":"{sourceCount}"}`); !strings.Contains(b, `"sourceCount":1`) {
		t.Fatalf("sourceCount: %s", b)
	}
	if b := gql(t, base, tok, `{"query":"{sourceCount(region:\"North America\")}"}`); !strings.Contains(b, `"sourceCount":1`) {
		t.Fatalf("sourceCount filtered: %s", b)
	}
	if b := gql(t, base, tok, `{"query":"{sourceCount(region:\"Nowhere\")}"}`); !strings.Contains(b, `"sourceCount":0`) {
		t.Fatalf("sourceCount empty: %s", b)
	}
	if b := gql(t, base, tok, `{"query":"{sources(limit:5,industry:\"Technology\"){id healthScore validationStatus languages tags}}"}`); !strings.Contains(b, `"validationStatus":"VALID"`) {
		t.Fatalf("filtered sources: %s", b)
	}
	if b := gql(t, base, tok, `{"query":"{sourceCoverage{byRegion{key count} byLanguage{key count} byScope{key count}}}"}`); !strings.Contains(b, "North America") {
		t.Fatalf("coverage: %s", b)
	}
	// Unauthenticated rejection of the new read resolvers.
	for _, q := range []string{`{"query":"{sourceCount}"}`, `{"query":"{sourceCoverage{byRegion{key}}}"}`} {
		if b := gql(t, base, "", q); !strings.Contains(b, "unauthenticated") {
			t.Fatalf("expected unauthenticated: %s -> %s", q, b)
		}
	}
}

func TestRevalidateSource(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()

	// A live RSS server and a dead endpoint.
	ref := time.Now()
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprint(w, feedDoc(ref))
	})
	mux.HandleFunc("/dead", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	feedSrv := httptest.NewServer(mux)
	defer feedSrv.Close()

	d.Pool.Exec(ctx, `INSERT INTO "Source" ("id","name","url","priority","credibility","crawlFrequency","updatedAt") VALUES ('rv1',$1,$2,2,0.5,900,now())`, "Revalidate Me", feedSrv.URL+"/ok")

	base := newServerWith(t, d, &recordEnqueuer{})
	admin, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	viewer, _ := dbtest.AuthToken(t, d, auth.RoleViewer)

	// Viewer forbidden.
	if b := gql(t, base, viewer, `{"query":"mutation{revalidateSource(id:\"rv1\"){id}}"}`); !strings.Contains(b, "forbidden") {
		t.Fatalf("viewer should be forbidden: %s", b)
	}
	// Admin revalidates a healthy feed → VALID with a log entry.
	if b := gql(t, base, admin, `{"query":"mutation{revalidateSource(id:\"rv1\"){validationStatus healthScore validationLogs{ok httpStatus itemCount}}}"}`); !strings.Contains(b, `"validationStatus":"VALID"`) || !strings.Contains(b, `"ok":true`) {
		t.Fatalf("revalidate VALID: %s", b)
	}
	// Point it at a dead endpoint and revalidate → INVALID.
	d.Pool.Exec(ctx, `UPDATE "Source" SET "url"=$1 WHERE "id"='rv1'`, feedSrv.URL+"/dead")
	if b := gql(t, base, admin, `{"query":"mutation{revalidateSource(id:\"rv1\"){validationStatus}}"}`); !strings.Contains(b, `"validationStatus":"INVALID"`) {
		t.Fatalf("revalidate INVALID: %s", b)
	}
	// Unknown id → null.
	if b := gql(t, base, admin, `{"query":"mutation{revalidateSource(id:\"nope\"){id}}"}`); !strings.Contains(b, `"revalidateSource":null`) {
		t.Fatalf("revalidate unknown: %s", b)
	}
}

func feedDoc(ref time.Time) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><rss version="2.0"><channel><title>T</title>`)
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&b, `<item><title>Item %d</title><link>http://x/%d</link><pubDate>%s</pubDate></item>`, i, i, ref.Add(-time.Hour).Format(time.RFC1123Z))
	}
	b.WriteString(`</channel></rss>`)
	return b.String()
}
