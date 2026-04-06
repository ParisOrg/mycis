package service

import (
	"errors"
	"testing"

	"mycis/internal/db"
)

func TestValidateEvidenceURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "https host", raw: "https://example.com/path?q=1", wantErr: false},
		{name: "https uppercase scheme", raw: "HTTPS://example.com/", wantErr: false},
		{name: "http rejected", raw: "http://example.com/", wantErr: true},
		{name: "javascript rejected", raw: "javascript:alert(1)", wantErr: true},
		{name: "no scheme", raw: "//example.com/", wantErr: true},
		{name: "relative", raw: "/foo", wantErr: true},
		{name: "empty host", raw: "https://", wantErr: true},
		{name: "newline", raw: "https://example.com/\n", wantErr: true},
		{name: "too long", raw: "https://example.com/" + string(make([]byte, maxEvidenceURLLength)), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ValidateEvidenceURL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("want ErrInvalidInput, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestParseAssessmentItemStatus(t *testing.T) {
	t.Parallel()
	got, err := ParseAssessmentItemStatus("in_progress")
	if err != nil || got != db.AssessmentItemStatusInProgress {
		t.Fatalf("got %v %v", got, err)
	}
	_, err = ParseAssessmentItemStatus("nope")
	if err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestParseItemPriority(t *testing.T) {
	t.Parallel()
	got, err := ParseItemPriority("high")
	if err != nil || got != db.ItemPriorityHigh {
		t.Fatalf("got %v %v", got, err)
	}
	_, err = ParseItemPriority("urgent")
	if err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestValidateEmailForStorage(t *testing.T) {
	t.Parallel()
	got, err := ValidateEmailForStorage("  User@Example.COM  ")
	if err != nil || got != "user@example.com" {
		t.Fatalf("got %q %v", got, err)
	}
	_, err = ValidateEmailForStorage("not an email")
	if err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	_, err = ValidateEmailForStorage("")
	if err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestNormalizeEmailForAuth(t *testing.T) {
	t.Parallel()
	got, err := NormalizeEmailForAuth("  A@B.CO  ")
	if err != nil || got != "a@b.co" {
		t.Fatalf("got %q %v", got, err)
	}
	_, err = NormalizeEmailForAuth("@@@")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestNormalizePassword(t *testing.T) {
	t.Parallel()

	got, err := normalizePassword("  password-12345  ")
	if err != nil || got != "password-12345" {
		t.Fatalf("got %q %v", got, err)
	}

	_, err = normalizePassword("short")
	if err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}
