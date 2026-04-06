package textutil

import "testing"

func TestTrimPtr(t *testing.T) {
	t.Parallel()

	if got := TrimPtr("   "); got != nil {
		t.Fatalf("expected nil, got %q", *got)
	}

	got := TrimPtr("  hello  ")
	if got == nil || *got != "hello" {
		t.Fatalf("got %v", got)
	}
}

func TestParseDateOnly(t *testing.T) {
	t.Parallel()

	date, err := ParseDateOnly("2026-04-05")
	if err != nil {
		t.Fatal(err)
	}
	if got := date.Format(DateLayout); got != "2026-04-05" {
		t.Fatalf("got %s", got)
	}

	if _, err := ParseDateOnly("2026/04/05"); err == nil {
		t.Fatal("expected parse error")
	}
}
