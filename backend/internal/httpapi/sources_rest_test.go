package httpapi_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func restPost(t *testing.T, base, path, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("POST", base+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	authHeaders(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func restPatch(t *testing.T, base, path, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("PATCH", base+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	authHeaders(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// TestSourcesREST exercises the legacy REST source endpoints end to end.
func TestSourcesREST(t *testing.T) {
	ht, _ := authServer(t)

	// Missing name/url → 400.
	if code, b := restPost(t, ht.URL, "/v1/sources", `{"url":"https://a.example/feed"}`); code != 400 || !strings.Contains(b, "required") {
		t.Fatalf("missing name: code=%d body=%s", code, b)
	}
	// Malformed JSON → 400 (readJSON unmarshal error).
	if code, _ := restPost(t, ht.URL, "/v1/sources", `{not json`); code != 400 {
		t.Fatalf("bad json: code=%d", code)
	}
	// Full create with all optional fields → 201.
	code, b := restPost(t, ht.URL, "/v1/sources", `{"name":"Acme","url":"https://acme.example/feed","type":"RSS","country":"US","priority":2,"crawlFrequency":600,"credibility":0.9}`)
	if code != 201 || !strings.Contains(b, `"Acme"`) {
		t.Fatalf("create: code=%d body=%s", code, b)
	}
	id := jsonField(b, "id")
	if id == "" {
		t.Fatalf("no id in create response: %s", b)
	}
	// Duplicate URL → 409.
	if code, b := restPost(t, ht.URL, "/v1/sources", `{"name":"Dup","url":"https://acme.example/feed"}`); code != 409 || !strings.Contains(b, "already exists") {
		t.Fatalf("dup url: code=%d body=%s", code, b)
	}
	// List → 200 with data.
	if code, b := restPost(t, ht.URL, "/v1/sources/"+id+"/fetch", ``); code != 200 || !strings.Contains(b, `"queued":true`) {
		t.Fatalf("fetch: code=%d body=%s", code, b)
	}
	// Patch with empty body (readJSON empty-body path) → 200.
	if code, _ := restPatch(t, ht.URL, "/v1/sources/"+id, ``); code != 200 {
		t.Fatalf("patch empty: code=%d", code)
	}
	// Patch toggling enabled/priority → 200.
	if code, b := restPatch(t, ht.URL, "/v1/sources/"+id, `{"enabled":false,"priority":7,"crawlFrequency":1200}`); code != 200 || !strings.Contains(b, id) {
		t.Fatalf("patch: code=%d body=%s", code, b)
	}
	// GET list.
	req, _ := http.NewRequest("GET", ht.URL+"/v1/sources", nil)
	authHeaders(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"data"`) {
		t.Fatalf("list: code=%d body=%s", resp.StatusCode, string(body))
	}
}

// jsonField pulls a string field value out of a flat JSON object body.
func jsonField(body, key string) string {
	marker := `"` + key + `":"`
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	rest := body[i+len(marker):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return ""
	}
	return rest[:j]
}
