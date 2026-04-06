package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadRequiresSessionKey(t *testing.T) {
	_, err := loadFromTempDir(t, map[string]string{
		"DATABASE_URL":    "postgres://example",
		"APP_SESSION_KEY": "",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "APP_SESSION_KEY is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsShortSessionKey(t *testing.T) {
	_, err := loadFromTempDir(t, map[string]string{
		"DATABASE_URL":    "postgres://example",
		"APP_SESSION_KEY": "too-short",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "at least 32 characters") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsExampleSessionKeys(t *testing.T) {
	for value := range disallowedSessionKeys {
		t.Run(value, func(t *testing.T) {
			_, err := loadFromTempDir(t, map[string]string{
				"DATABASE_URL":    "postgres://example",
				"APP_SESSION_KEY": value,
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "replaced from the example value") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadAcceptsValidSessionKey(t *testing.T) {
	cfg, err := loadFromTempDir(t, map[string]string{
		"DATABASE_URL":    "postgres://example",
		"APP_SESSION_KEY": "0123456789abcdef0123456789abcdef",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SessionKey != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("unexpected session key: %q", cfg.SessionKey)
	}
}

func loadFromTempDir(t *testing.T, env map[string]string) (Config, error) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
	})

	for key, value := range env {
		t.Setenv(key, value)
	}

	return Load()
}
