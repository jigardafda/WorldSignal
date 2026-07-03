package email

import (
	"strings"
	"testing"
)

func TestRenderSignal(t *testing.T) {
	country := "US"
	c := SignalCard{
		ID: "sig1", Title: "Quake hits <coast>", Summary: "A strong quake.", Severity: "HIGH",
		Country: country, Tags: []string{"disaster.earthquake"}, SourceCount: 3, WhenText: "2h ago",
	}
	subject, text, html := RenderSignal(c, Branding{BaseURL: "https://ws.example.com"})
	if !strings.HasPrefix(subject, "[HIGH]") {
		t.Errorf("subject: %q", subject)
	}
	if !strings.Contains(text, "Quake hits <coast>") || !strings.Contains(text, "Severity: HIGH") {
		t.Errorf("text missing content:\n%s", text)
	}
	if !strings.Contains(text, "https://ws.example.com/signals/sig1") {
		t.Errorf("text should include console link:\n%s", text)
	}
	// HTML must escape the angle brackets in the title.
	if strings.Contains(html, "<coast>") || !strings.Contains(html, "&lt;coast&gt;") {
		t.Errorf("html not escaped:\n%s", html)
	}
	if !strings.Contains(html, "HIGH") {
		t.Error("html missing severity badge")
	}
}

func TestRenderSignalPrefersArticleLink(t *testing.T) {
	c := SignalCard{ID: "s", Title: "T", Severity: "LOW", Link: "https://news.example/article"}
	_, text, _ := RenderSignal(c, Branding{BaseURL: "https://ws.example.com"})
	if !strings.Contains(text, "https://news.example/article") {
		t.Errorf("should prefer article link:\n%s", text)
	}
}

func TestRenderDigest(t *testing.T) {
	link := "https://news.example/a"
	cards := []SignalCard{
		{ID: "1", Title: "First", Severity: "CRITICAL", Country: "IN", SourceCount: 2, Link: link, WhenText: "1h ago"},
		{ID: "2", Title: "Second", Severity: "MEDIUM"},
	}
	subject, text, html := RenderDigest(cards, "daily", Branding{AppName: "WS"})
	if !strings.Contains(subject, "Daily digest") || !strings.Contains(subject, "2 new signals") {
		t.Errorf("subject: %q", subject)
	}
	if !strings.Contains(text, "1. [CRITICAL] First") || !strings.Contains(text, "2. [MEDIUM] Second") {
		t.Errorf("text:\n%s", text)
	}
	if !strings.Contains(html, "First") || !strings.Contains(html, "Second") {
		t.Error("html missing signals")
	}
	// Singular form.
	sub1, _, _ := RenderDigest(cards[:1], "hourly", Branding{})
	if !strings.Contains(sub1, "1 new signal") || strings.Contains(sub1, "1 new signals") {
		t.Errorf("singular subject: %q", sub1)
	}
	if !strings.Contains(sub1, "Hourly") {
		t.Errorf("interval label: %q", sub1)
	}
}

func TestBrandingDefaults(t *testing.T) {
	b := Branding{}
	if b.appName() != "WorldSignal" {
		t.Errorf("default app name: %q", b.appName())
	}
	if b.signalURL("x") != "" {
		t.Error("no base URL should yield no link")
	}
	if got := (Branding{BaseURL: "https://x/"}).signalURL("id"); got != "https://x/signals/id" {
		t.Errorf("signalURL: %q", got)
	}
}

func TestProviders(t *testing.T) {
	provs := Providers()
	if len(provs) < 4 {
		t.Fatalf("expected several presets, got %d", len(provs))
	}
	if provs[0].Code != ProviderGmail {
		t.Errorf("gmail should be first, got %s", provs[0].Code)
	}
	for _, code := range []string{ProviderGmail, ProviderOutlook, ProviderZoho, ProviderSendGrid, ProviderCustom} {
		p, ok := Preset(code)
		if !ok {
			t.Errorf("missing preset %s", code)
		}
		if code != ProviderCustom && p.Host == "" {
			t.Errorf("%s should have a default host", code)
		}
	}
	if _, ok := Preset("nope"); ok {
		t.Error("unknown provider should not resolve")
	}
	if !ValidProvider("gmail") { // case-insensitive
		t.Error("ValidProvider should be case-insensitive")
	}
}
