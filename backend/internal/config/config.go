package config

import (
	"os"
	"time"
)

// Config holds all runtime configuration for rozszerzify.
// NOTE: personal values (dates, secrets) are env-only — nothing personal is
// ever committed to this public repo. Empty StartDate/BirthDate are handled
// gracefully (stats omit the related counters).
type Config struct {
	DatabaseURL  string
	JWTSecret    string
	ListenAddr   string
	PublicURL    string
	StartDate    string // ISO date — when diet expansion starts (env)
	BirthDate    string // ISO date — baby's birthday (env)
	SeedPassword string
	RemindDir    string // marker files for the -remind cron job

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
		StartDate:    getEnv("START_DATE", ""),
		BirthDate:    getEnv("BIRTH_DATE", ""),
		SeedPassword: getEnv("SEED_PASSWORD", ""),
		RemindDir:    getEnv("REMIND_DIR", "/opt/rozszerzify"),

		PushoverUserKey:  getEnv("PUSHOVER_USER_KEY", ""),
		PushoverAppToken: getEnv("PUSHOVER_APP_TOKEN", ""),
	}
}

// StartTime returns the parsed diet-expansion start date at 00:00 UTC.
// Returns a zero time when unset/unparseable (stats then show a countdown
// only if a start date is actually configured).
func (c *Config) StartTime() time.Time {
	if c.StartDate == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", c.StartDate)
	if err != nil {
		return time.Time{}
	}
	return t
}

// BirthTime returns the parsed birth date at 00:00 UTC (zero when unset).
func (c *Config) BirthTime() time.Time {
	if c.BirthDate == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", c.BirthDate)
	if err != nil {
		return time.Time{}
	}
	return t
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}