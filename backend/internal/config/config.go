// Package config ports backend/src/config/env.ts. Same variables, defaults and
// validation (DATABASE_URL required; ROLE constrained to all|api|worker).
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the validated environment.
type Config struct {
	DatabaseURL          string
	Port                 int
	Host                 string
	OpenAIAPIKey         string
	OpenAIModel          string
	Role                 string
	WebhookSigningSecret string
	SchedulerTickMS      int
	AdminEmail           string
	AdminPassword        string
	// Source failure handling.
	SourceFailureThreshold int // consecutive failures before cooldown
	SourceCooldownMinutes  int // cooldown duration once threshold is hit
	// AppBaseURL is the public console URL (e.g. https://signals.example.com). When
	// set, emails link back to signals in the console. Optional.
	AppBaseURL string
	// DigestTickSeconds is how often the digest scheduler checks for due digests.
	DigestTickSeconds int
}

// HasOpenAI reports whether an OpenAI key is configured.
func (c Config) HasOpenAI() bool { return len(c.OpenAIAPIKey) > 0 }

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// Load reads and validates the environment, mirroring the zod schema.
func Load() (Config, error) {
	c := Config{
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		Host:                 getenv("HOST", "0.0.0.0"),
		OpenAIAPIKey:         getenv("OPENAI_API_KEY", ""),
		OpenAIModel:          getenv("OPENAI_MODEL", "gpt-4o-mini"),
		Role:                 getenv("ROLE", "all"),
		WebhookSigningSecret: getenv("WEBHOOK_SIGNING_SECRET", "change-me-in-prod"),
		AdminEmail:           getenv("ADMIN_EMAIL", "admin@worldsignal.local"),
		AdminPassword:        getenv("ADMIN_PASSWORD", "admin12345"),
		AppBaseURL:           getenv("APP_BASE_URL", ""),
	}

	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	port, err := strconv.Atoi(getenv("PORT", "4800"))
	if err != nil {
		return Config{}, fmt.Errorf("PORT must be a number: %w", err)
	}
	c.Port = port

	switch c.Role {
	case "all", "api", "worker":
	default:
		return Config{}, fmt.Errorf("ROLE must be one of all|api|worker, got %q", c.Role)
	}

	tick, err := strconv.Atoi(getenv("SCHEDULER_TICK_MS", "30000"))
	if err != nil {
		return Config{}, fmt.Errorf("SCHEDULER_TICK_MS must be a number: %w", err)
	}
	c.SchedulerTickMS = tick

	ft, err := strconv.Atoi(getenv("SOURCE_FAILURE_THRESHOLD", "5"))
	if err != nil || ft < 1 {
		return Config{}, fmt.Errorf("SOURCE_FAILURE_THRESHOLD must be a positive number")
	}
	c.SourceFailureThreshold = ft

	cd, err := strconv.Atoi(getenv("SOURCE_COOLDOWN_MINUTES", "180"))
	if err != nil || cd < 1 {
		return Config{}, fmt.Errorf("SOURCE_COOLDOWN_MINUTES must be a positive number")
	}
	c.SourceCooldownMinutes = cd

	dt, err := strconv.Atoi(getenv("DIGEST_TICK_SECONDS", "60"))
	if err != nil || dt < 1 {
		return Config{}, fmt.Errorf("DIGEST_TICK_SECONDS must be a positive number")
	}
	c.DigestTickSeconds = dt

	return c, nil
}
