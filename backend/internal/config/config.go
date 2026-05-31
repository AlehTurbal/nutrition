// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds all runtime settings for the server.
type Config struct {
	Port            string
	DatabaseURL     string
	JWTSecret       string
	JWTTTL          time.Duration
	AnthropicAPIKey string
	LLMModel        string
}

// Load reads configuration from the environment, applying sensible defaults for
// local development. DatabaseURL and JWTSecret are required.
func Load() (Config, error) {
	cfg := Config{
		Port:        getenv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		JWTTTL:      24 * time.Hour,
	}

	if ttl := os.Getenv("JWT_TTL"); ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil {
			return Config{}, fmt.Errorf("invalid JWT_TTL: %w", err)
		}
		cfg.JWTTTL = d
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	cfg.AnthropicAPIKey = os.Getenv("ANTHROPIC_API_KEY")
	cfg.LLMModel = getenv("LLM_MODEL", "claude-opus-4-8")

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
