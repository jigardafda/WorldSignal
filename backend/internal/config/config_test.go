package config

import "testing"

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"DATABASE_URL", "PORT", "HOST", "OPENAI_API_KEY", "OPENAI_MODEL", "ROLE", "WEBHOOK_SIGNING_SECRET", "SCHEDULER_TICK_MS"} {
		t.Setenv(k, "")
		// t.Setenv sets to ""; for "unset" semantics we Setenv then rely on defaults
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("PORT", "")
	// Unset optional ones by removing — use os via Setenv to default path:
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "0.0.0.0" || c.OpenAIModel != "gpt-4o-mini" || c.Role != "all" {
		t.Fatalf("bad defaults: %+v", c)
	}
	if c.WebhookSigningSecret != "change-me-in-prod" {
		t.Fatalf("bad secret default: %q", c.WebhookSigningSecret)
	}
	if c.HasOpenAI() {
		t.Fatal("HasOpenAI should be false with empty key")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL missing")
	}
}

func TestLoadInvalidRole(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("ROLE", "bogus")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid ROLE")
	}
}

func TestLoadInvalidPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("ROLE", "all")
	t.Setenv("PORT", "abc")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid PORT")
	}
}

func TestLoadInvalidTick(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("ROLE", "all")
	t.Setenv("PORT", "4000")
	t.Setenv("SCHEDULER_TICK_MS", "xx")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid SCHEDULER_TICK_MS")
	}
}

func TestLoadCustomValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("PORT", "8080")
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_MODEL", "gpt-x")
	t.Setenv("ROLE", "worker")
	t.Setenv("WEBHOOK_SIGNING_SECRET", "s3cret")
	t.Setenv("SCHEDULER_TICK_MS", "1000")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 8080 || c.Host != "127.0.0.1" || c.Role != "worker" || c.SchedulerTickMS != 1000 {
		t.Fatalf("bad custom values: %+v", c)
	}
	if !c.HasOpenAI() {
		t.Fatal("HasOpenAI should be true")
	}
}
