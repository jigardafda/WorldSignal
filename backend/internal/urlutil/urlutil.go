// Package urlutil ports backend/src/lib/url.ts: aggressive URL canonicalization
// so the same article from different places dedupes cleanly. Output is built to
// match the WHATWG URL serialization the TS code relies on.
package urlutil

import (
	"net/url"
	"strings"
)

var trackingParams = map[string]struct{}{
	"utm_source": {}, "utm_medium": {}, "utm_campaign": {}, "utm_term": {},
	"utm_content": {}, "fbclid": {}, "gclid": {}, "mc_cid": {}, "mc_eid": {},
	"ref": {}, "ref_src": {}, "igshid": {}, "_hsenc": {}, "_hsmi": {}, "spm": {},
}

// Canonicalize mirrors canonicalizeUrl(). The bool is false when the TS function
// would return null (empty/whitespace-only input).
func Canonicalize(input string) (string, bool) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	// WHATWG `new URL()` throws without a scheme+host; the TS code then keeps the
	// raw string. net/url is lenient, so detect that case explicitly.
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw, true
	}

	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")

	if port := u.Port(); port != "" && !isDefaultPort(scheme, port) {
		host += ":" + port
	}

	// Drop tracking params, then re-serialize sorted (url.Values.Encode sorts keys).
	q := u.Query()
	for p := range trackingParams {
		q.Del(p)
	}
	query := q.Encode()

	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimRight(path, "/")
		if path == "" {
			path = "/"
		}
	}

	// Google News RSS item links use the feed-only "/rss/articles/<token>" form,
	// which shows a "Redirect notice"/invalid-address error in a browser. The
	// browser-resolvable form is "/articles/<token>".
	if host == "news.google.com" && strings.HasPrefix(path, "/rss/articles/") {
		path = strings.TrimPrefix(path, "/rss")
	}

	out := scheme + "://" + host + path
	if query != "" {
		out += "?" + query
	}
	return out, true
}

func isDefaultPort(scheme, port string) bool {
	return (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
}
