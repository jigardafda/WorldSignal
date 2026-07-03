package email

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMessageIDAndBoundary(t *testing.T) {
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if id := newMessageID("a@example.com", when); !strings.HasSuffix(id, "@example.com>") || !strings.HasPrefix(id, "<") {
		t.Errorf("message id: %q", id)
	}
	// No @ → falls back to the default domain.
	if id := newMessageID("noatsign", when); !strings.Contains(id, "@worldsignal.local>") {
		t.Errorf("fallback message id: %q", id)
	}
	if b := newBoundary(when); !strings.HasPrefix(b, "ws_") {
		t.Errorf("boundary: %q", b)
	}
}

func TestSevColorDefault(t *testing.T) {
	if sevColor("HIGH") == sevColor("UNKNOWN_SEV") {
		t.Error("known and unknown severities should differ in color")
	}
}

func TestSevTitleAndInterval(t *testing.T) {
	if sevTitle("") != "SIGNAL" {
		t.Error("empty severity should render as SIGNAL")
	}
	if sevTitle("weird") != "WEIRD" {
		t.Error("unknown severity should upper-case")
	}
	if intervalLabel("weekly") != "Daily" {
		t.Error("unknown interval falls back to Daily")
	}
}

func TestOneLine(t *testing.T) {
	if oneLine("a\nb\r c") != "a b  c" {
		t.Errorf("oneLine: %q", oneLine("a\nb\r c"))
	}
}

func TestMetaLineEmpty(t *testing.T) {
	// A card with only an (empty) severity still yields the severity token.
	c := SignalCard{Severity: ""}
	if c.metaLine() != "SIGNAL" {
		t.Errorf("metaLine: %q", c.metaLine())
	}
}

func TestSendTLSPath(t *testing.T) {
	// Point implicit-TLS mode at a non-TLS listener: the TLS handshake must fail,
	// exercising the SecurityTLS dial branch.
	f := newFakeSMTP(t)
	cfg := cfgFor(f)
	cfg.Security = SecurityTLS
	if err := Verify(context.Background(), cfg); err == nil {
		t.Fatal("expected TLS handshake failure against plaintext server")
	}
}
