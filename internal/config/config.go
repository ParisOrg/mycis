package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName      string
	Addr         string
	DatabaseURL  string
	SessionKey   string
	CookieSecure bool
}

var disallowedSessionKeys = map[string]struct{}{
	"change-me-in-production-change-me-in-production":          {},
	"dev-only-change-this-to-a-long-random-secret-32chars-min": {},
	"dev-session-key-change-me-dev-session-key":                {},
	"replace-with-a-long-random-session-key":                   {},
	"replace-me-with-a-32-byte-random-session-key":             {},
}

func Load() (Config, error) {
	_ = godotenv.Load(".env")

	cfg := Config{
		AppName:      getenv("APP_NAME", "Controls Tracker"),
		Addr:         getenv("APP_ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		SessionKey:   strings.TrimSpace(os.Getenv("APP_SESSION_KEY")),
		CookieSecure: getbool("APP_COOKIE_SECURE", false),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.SessionKey == "" {
		return Config{}, fmt.Errorf("APP_SESSION_KEY is required")
	}

	if len(cfg.SessionKey) < 32 {
		return Config{}, fmt.Errorf("APP_SESSION_KEY must be at least 32 characters")
	}

	if _, blocked := disallowedSessionKeys[cfg.SessionKey]; blocked {
		return Config{}, fmt.Errorf("APP_SESSION_KEY must be replaced from the example value")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getbool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
