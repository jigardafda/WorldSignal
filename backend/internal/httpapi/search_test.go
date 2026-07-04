package httpapi_test

import (
	"context"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

// seedSearchable seeds two signals with distinct text and typed entities.
func seedSearchable(t *testing.T, d *db.DB) {
	t.Helper()
	dbtest.Reset(t, d)
	ctx := context.Background()
	ex := func(q string, a ...any) {
		if _, err := d.Pool.Exec(ctx, q, a...); err != nil {
			t.Fatal(err)
		}
	}
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('q','Major earthquake strikes city','damage reported',now(),now(),now())`)
	ex(`INSERT INTO "Signal" ("id","title","summary","firstSeenAt","lastSeenAt","updatedAt") VALUES ('e','Economic outlook','calm markets',now(),now(),now())`)
	ex(`INSERT INTO "SignalAttribute" ("signalId","key","valueCode","valueText","confidence") VALUES
	    ('q','entity','ORG','Red Cross',1),('q','entity','LOCATION','Coastal City',1),('e','entity','ORG','Central Bank',1)`)
}

func TestGraphQLSearchAndEntities(t *testing.T) {
	d := dbtest.Connect(t)
	seedSearchable(t, d)
	ht, _ := newServer(t, d)
	tok, _ := dbtest.AuthToken(t, d, "VIEWER")
	bearer = tok
	defer func() { bearer = "" }()

	// Full-text signal search returns only the earthquake signal.
	_, body := postGQL(t, ht.URL, `{"query":"{ signals(filter:{search:\"earthquake\"}){ id } }"}`)
	if !strings.Contains(body, `"id":"q"`) || strings.Contains(body, `"id":"e"`) {
		t.Fatalf("search: %s", body)
	}
	// Entity filter on signals.
	_, body = postGQL(t, ht.URL, `{"query":"{ signals(filter:{entity:\"Red Cross\"}){ id } }"}`)
	if !strings.Contains(body, `"id":"q"`) || strings.Contains(body, `"id":"e"`) {
		t.Fatalf("entity filter: %s", body)
	}
	// Entities query — searchable + typed with counts.
	_, body = postGQL(t, ht.URL, `{"query":"{ entities(search:\"c\"){ name type signalCount } }"}`)
	for _, want := range []string{`"name":"Central Bank"`, `"name":"Coastal City"`, `"type":"ORG"`, `"signalCount":1`} {
		if !strings.Contains(body, want) {
			t.Fatalf("entities missing %s: %s", want, body)
		}
	}
	// Type filter.
	_, body = postGQL(t, ht.URL, `{"query":"{ entities(type:\"LOCATION\"){ name } }"}`)
	if !strings.Contains(body, `"name":"Coastal City"`) || strings.Contains(body, `"Central Bank"`) {
		t.Fatalf("entity type filter: %s", body)
	}
}

func TestEntitiesAuthz(t *testing.T) {
	d := dbtest.Connect(t)
	seedSearchable(t, d)
	ht, _ := newServer(t, d)
	// No bearer → unauthenticated.
	bearer = ""
	if _, body := postGQL(t, ht.URL, `{"query":"{ entities{ name } }"}`); !strings.Contains(body, "unauthenticated") {
		t.Fatalf("expected unauthenticated: %s", body)
	}
}

func TestEntitiesDBError(t *testing.T) {
	d := dbtest.Connect(t)
	seedSearchable(t, d)
	ht, _ := newServer(t, d)
	tok, _ := dbtest.AuthToken(t, d, "VIEWER")
	bearer = tok
	defer func() { bearer = "" }()
	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "SignalAttribute" RENAME TO "SignalAttribute__h"`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.Pool.Exec(ctx, `ALTER TABLE "SignalAttribute__h" RENAME TO "SignalAttribute"`) }()

	if _, body := postGQL(t, ht.URL, `{"query":"{ entities{ name } }"}`); !strings.Contains(body, `"errors"`) {
		t.Fatalf("expected gql error: %s", body)
	}
	if code, _ := get(t, ht.URL, "/v1/entities"); code != 500 {
		t.Fatalf("expected 500, got %d", code)
	}
}

func TestRESTEntitiesAndSearch(t *testing.T) {
	d := dbtest.Connect(t)
	seedSearchable(t, d)
	ht, _ := newServer(t, d)

	// REST entity index.
	if code, body := get(t, ht.URL, "/v1/entities?search=red"); code != 200 || !strings.Contains(body, `"name":"Red Cross"`) {
		t.Fatalf("/v1/entities: %d %s", code, body)
	}
	if _, body := get(t, ht.URL, "/v1/entities?type=LOCATION&limit=10"); !strings.Contains(body, `"Coastal City"`) {
		t.Fatalf("/v1/entities type: %s", body)
	}
	// REST signal search + entity filter.
	if _, body := get(t, ht.URL, "/v1/signals?search=earthquake"); !strings.Contains(body, `"id":"q"`) || strings.Contains(body, `"id":"e"`) {
		t.Fatalf("/v1/signals search: %s", body)
	}
	if _, body := get(t, ht.URL, "/v1/signals?entity=Red%20Cross"); !strings.Contains(body, `"id":"q"`) {
		t.Fatalf("/v1/signals entity: %s", body)
	}
}
