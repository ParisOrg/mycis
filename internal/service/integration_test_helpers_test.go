package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mycis/internal/db"
)

type integrationHarness struct {
	ctx      context.Context
	rootDir  string
	services *Services
	queries  *db.Queries
}

func newIntegrationHarness(t *testing.T) *integrationHarness {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	rootDir := repoRoot(t)
	withWorkingDirectory(t, rootDir)

	testDatabaseURL := createIntegrationDatabase(t, ctx, databaseURL)
	runMigrationsForTest(t, testDatabaseURL, rootDir)

	pool, err := pgxpool.New(ctx, testDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	return &integrationHarness{
		ctx:      ctx,
		rootDir:  rootDir,
		services: New(pool),
		queries:  db.New(pool),
	}
}

func (h *integrationHarness) createUser(t *testing.T, name, email string, isAdmin bool) db.User {
	t.Helper()

	user, err := h.services.Auth.CreateUserWithPassword(h.ctx, name, email, "password-12345", isAdmin, false)
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func (h *integrationHarness) createAdmin(t *testing.T) db.User {
	t.Helper()

	return h.createUser(t, "Admin", "admin@example.com", true)
}

func (h *integrationHarness) onlyFramework(t *testing.T) db.ListFrameworksWithCountsRow {
	t.Helper()

	frameworks, err := h.services.Frameworks.ListFrameworks(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(frameworks) != 1 {
		t.Fatalf("expected one framework, got %d", len(frameworks))
	}
	return frameworks[0]
}

func (h *integrationHarness) writeFrameworkSeed(t *testing.T, slug, contents string) {
	t.Helper()

	seedPath := filepath.Join(h.rootDir, "seed", "frameworks", slug+".yaml")
	if err := os.WriteFile(seedPath, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(seedPath)
	})
}

func findAssessmentItem(t *testing.T, items []db.ListAssessmentItemsRow, frameworkItemID uuid.UUID) db.ListAssessmentItemsRow {
	t.Helper()

	for _, item := range items {
		if item.FrameworkItemID == frameworkItemID {
			return item
		}
	}

	t.Fatalf("assessment item for framework item %s not found", frameworkItemID)
	return db.ListAssessmentItemsRow{}
}

func findAssessmentItemByCode(t *testing.T, items []db.ListAssessmentItemsRow, itemCode string) db.ListAssessmentItemsRow {
	t.Helper()

	for _, item := range items {
		if item.ItemCode == itemCode {
			return item
		}
	}

	t.Fatalf("assessment item with code %s not found", itemCode)
	return db.ListAssessmentItemsRow{}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine caller path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func withWorkingDirectory(t *testing.T, dir string) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
}

func createIntegrationDatabase(t *testing.T, ctx context.Context, baseDatabaseURL string) string {
	t.Helper()

	adminPool, err := pgxpool.New(ctx, baseDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}

	databaseName := fmt.Sprintf("mycis_smoke_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, `CREATE DATABASE "`+databaseName+`"`); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := adminPool.Exec(context.Background(), `DROP DATABASE IF EXISTS "`+databaseName+`" WITH (FORCE)`); err != nil {
			t.Errorf("drop smoke database: %v", err)
		}
		adminPool.Close()
	})

	parsedURL, err := url.Parse(baseDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	parsedURL.Path = "/" + databaseName
	return parsedURL.String()
}

func runMigrationsForTest(t *testing.T, databaseURL, rootDir string) {
	t.Helper()

	sourceURL := "file://" + filepath.Join(rootDir, "db", "migrations")
	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatal(err)
	}
}

func assessmentItemCodes(items []db.ListAssessmentItemsRow) []string {
	codes := make([]string, 0, len(items))
	for _, item := range items {
		codes = append(codes, item.ItemCode)
	}
	return codes
}

func testFrameworkSeedYAML(slug, version string, itemCodes ...string) string {
	var builder strings.Builder
	builder.WriteString("framework:\n")
	fmt.Fprintf(&builder, "  slug: %s\n", slug)
	builder.WriteString("  label: Test Framework\n")
	fmt.Fprintf(&builder, "  version: %s\n", version)
	builder.WriteString("groups:\n")
	builder.WriteString("  - code: \"1\"\n")
	builder.WriteString("    title: Group 1\n")
	builder.WriteString("    summary: Test summary\n")
	builder.WriteString("    description: Test description\n")
	builder.WriteString("items:\n")
	for _, code := range itemCodes {
		builder.WriteString("  - group_code: \"1\"\n")
		fmt.Fprintf(&builder, "    code: \"%s\"\n", code)
		fmt.Fprintf(&builder, "    title: Control %s\n", code)
		fmt.Fprintf(&builder, "    summary: Summary %s\n", code)
		fmt.Fprintf(&builder, "    description: Description %s\n", code)
		builder.WriteString("    asset_class: Devices\n")
		builder.WriteString("    security_function: Identify\n")
		builder.WriteString("    tags:\n")
		builder.WriteString("      - ig1\n")
	}
	return builder.String()
}

func testOrderedFrameworkSeedYAML(slug string) string {
	return fmt.Sprintf(`framework:
  slug: %s
  label: Ordered Framework
  version: 1.0.0
groups:
  - code: "ZZ"
    title: Group ZZ
    summary: Group ZZ summary
    description: Group ZZ description
  - code: "AA"
    title: Group AA
    summary: Group AA summary
    description: Group AA description
  - code: "MM"
    title: Group MM
    summary: Group MM summary
    description: Group MM description
items:
  - group_code: "ZZ"
    code: "ZZ-two"
    title: Control ZZ-two
    summary: Control ZZ-two
    description: Control ZZ-two
    asset_class: Systems
    security_function: Govern
    tags:
      - alpha
  - group_code: "AA"
    code: "AA-one"
    title: Control AA-one
    summary: Control AA-one
    description: Control AA-one
    asset_class: Systems
    security_function: Govern
    tags:
      - beta
  - group_code: "MM"
    code: "MM-three"
    title: Control MM-three
    summary: Control MM-three
    description: Control MM-three
    asset_class: Systems
    security_function: Govern
    tags:
      - gamma
  - group_code: "AA"
    code: "AA-nine"
    title: Control AA-nine
    summary: Control AA-nine
    description: Control AA-nine
    asset_class: Systems
    security_function: Govern
    tags:
      - delta
`, slug)
}
