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
	StartDate    string // ISO date, e.g. 2025-12-15 (when diet expansion started)
	BirthDate    string // ISO date, e.g. 2025-06-15 (baby's birthday — age counter)
	SeedPassword string

	// Pushover (optional — notifications disabled unless both keys are set)
	PushoverUserKey  string
	PushoverAppToken string
}

// Load reads configuration from environment variables.
func Load() *Config {
	return &Config{
		DatabaseURL:  getEnv("DATABASE_URL", ""),
		JWTSecret:    getEnv("JWT_SECRET", "change-me-secret"),
		ListenAddr:   getEnv("LISTEN_ADDR", "127.0.0.1:8081"),
		PublicURL:    getEnv("PUBLIC_URL", "https://rozszerzify.dom3k.pl"),
		StartDate:    getEnv("START_DATE", "2025-12-15"),
		BirthDate:    getEnv("BIRTH_DATE", "2025-06-15"),
		SeedPassword: getEnv("SEED_PASSWORD", ""),

		PushoverUserKey:  getEnv("PUSHOVER_USER_KEY", ""),
		PushoverAppToken: getEnv("PUSHOVER_APP_TOKEN", ""),
	}
}

// StartTime returns the parsed diet-expansion start date at 00:00 UTC.
// On parse failure it falls back to the default date.
func (c *Config) StartTime() time.Time {
	t, err := time.Parse("2006-01-02", c.StartDate)
	if err != nil {
		return time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)
	}
	return t
}

// BirthTime returns the parsed birth date at 00:00 UTC.
func (c *Config) BirthTime() time.Time {
	t, err := time.Parse("2006-01-02", c.BirthDate)
	if err != nil {
		return time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	}
	return t
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}