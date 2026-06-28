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
	}

	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	port, err := strconv.Atoi(getenv("PORT", "4000"))
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

	return c, nil
}
