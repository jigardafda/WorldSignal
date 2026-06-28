package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIGatewayDisabled(t *testing.T) {
	gw := NewOpenAIGateway("", "gpt-4o-mini")
	if gw.Enabled() {
		t.Fatal("empty key should be disabled")
	}
	out, err := gw.JSONCompletion(context.Background(), "s", "u", 100)
	if out != nil || err != nil {
		t.Fatalf("disabled should return nil,nil; got %v,%v", out, err)
	}
}

func TestOpenAIGatewaySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("missing auth header")
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"title\":\"T\"}"}}]}`))
	}))
	defer srv.Close()

	gw := NewOpenAIGateway("sk-test", "gpt-4o-mini")
	gw.BaseURL = srv.URL
	out, err := gw.JSONCompletion(context.Background(), "s", "u", 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"title":"T"}` {
		t.Fatalf("got %s", out)
	}
}

func TestOpenAIGatewayNilClientUsesDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer srv.Close()
	gw := NewOpenAIGateway("sk-test", "m")
	gw.Client = nil // exercises the http.DefaultClient fallback
	gw.BaseURL = srv.URL
	if _, err := gw.JSONCompletion(context.Background(), "s", "u", 5); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAIGatewayEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()
	gw := NewOpenAIGateway("sk-test", "m")
	gw.BaseURL = srv.URL
	out, _ := gw.JSONCompletion(context.Background(), "s", "u", 10)
	if out != nil {
		t.Fatalf("empty choices should yield nil, got %s", out)
	}
}

func TestOpenAIGatewayBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	gw := NewOpenAIGateway("sk-test", "m")
	gw.BaseURL = srv.URL
	out, _ := gw.JSONCompletion(context.Background(), "s", "u", 10)
	if out != nil {
		t.Fatalf("bad json should yield nil, got %s", out)
	}
}

func TestOpenAIGatewayConnError(t *testing.T) {
	gw := NewOpenAIGateway("sk-test", "m")
	gw.BaseURL = "http://127.0.0.1:0" // unconnectable
	out, err := gw.JSONCompletion(context.Background(), "s", "u", 10)
	if out != nil || err != nil {
		t.Fatalf("conn error should fall back to nil,nil; got %v,%v", out, err)
	}
}

func TestBuildTaxonomyListNonEmpty(t *testing.T) {
	if buildTaxonomyList() == "" {
		t.Fatal("taxonomy list should not be empty")
	}
}

func TestDynamicGateway(t *testing.T) {
	// A server that echoes a minimal chat-completion response.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer dyn-key" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"ok\":true}"}}]}`))
	}))
	defer srv.Close()

	// Resolver returns a key+model → gateway is enabled and delegates.
	key := "dyn-key"
	g := NewDynamicGateway(func(ctx context.Context) (string, string) { return key, "gpt-4o" })
	g.BaseURL = srv.URL
	if !g.Enabled() {
		t.Fatal("expected enabled when resolver returns a key")
	}
	out, err := g.JSONCompletion(context.Background(), "s", "u", 50)
	if err != nil || string(out) != `{"ok":true}` {
		t.Fatalf("dynamic completion: out=%s err=%v", out, err)
	}

	// Empty key → disabled, completion returns nil.
	key = ""
	if g.Enabled() {
		t.Fatal("expected disabled when resolver returns empty key")
	}
	if out, err := g.JSONCompletion(context.Background(), "s", "u", 50); out != nil || err != nil {
		t.Fatalf("disabled dynamic should return nil,nil; got %s,%v", out, err)
	}
}
