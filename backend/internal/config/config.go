package config

import (
	"os"
	"time"
)

// Config holds all runtime configuration for rozszerzify.
type Config struct {
	DatabaseURL  string
	JWTSecret    string
	ListenAddr   string
	PublicURL    string
	StartDate    string // ISO date, e.g. 2025-12-15
	SeedPassword string
}

// Load reads configuration from environment variables.
func Load() *Config {
	return &Config{
		DatabaseURL:  getEnv("DATABASE_URL", ""),
		JWTSecret:    getEnv("JWT_SECRET", "change-me-secret"),
		ListenAddr:   getEnv("LISTEN_ADDR", "127.0.0.1:8081"),
		PublicURL:    getEnv("PUBLIC_URL", "https://rozszerzify.dom3k.pl"),
		StartDate:    getEnv("START_DATE", "2025-12-15"),
		SeedPassword: getEnv("SEED_PASSWORD", ""),
	}
}

// StartTime returns the parsed start date at 00:00 UTC.
// On parse failure it logs nothing and falls back to the default date.
func (c *Config) StartTime() time.Time {
	t, err := time.Parse("2006-01-02", c.StartDate)
	if err != nil {
		return time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)
	}
	return t
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}