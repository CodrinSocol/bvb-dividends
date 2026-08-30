// Package pgtest points integration tests at a real database.
//
// A test that uses it skips when TEST_DATABASE_URL is unset, so `go test ./...`
// still works on a machine with no database; CI always sets it.
package pgtest

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/postgres"
)

// URLEnv names the variable the integration tests are pointed at a database
// with.
const URLEnv = "TEST_DATABASE_URL"

// NewDB connects to the test database, applies the given namespaces' schemas,
// and returns the result.
//
// The tests truncate between cases, so pointing them at a database somebody is
// using would silently destroy its contents. Requiring the name to say it is a
// test database makes that mistake impossible rather than merely unlikely.
func NewDB(t *testing.T, namespaces ...*postgres.Namespace) *postgres.DB {
	t.Helper()

	connectionURL := os.Getenv(URLEnv)
	if connectionURL == "" {
		t.Skipf("%s is not set", URLEnv)
	}

	cfg, err := postgres.ConfigFromURL(connectionURL)
	if err != nil {
		t.Fatalf("%s: %v", URLEnv, err)
	}
	if !strings.Contains(cfg.Database, "test") {
		t.Fatalf("refusing to run destructive tests against %q: %s must name a database with %q in it",
			cfg.Database, URLEnv, "test")
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	db, err := postgres.NewDBWithConfig(t.Context(), log, cfg, namespaces)
	if err != nil {
		t.Fatalf("connect to the test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Shutdown(t.Context()) })

	if err := db.Start(t.Context()); err != nil {
		t.Fatalf("migrate the test database: %v", err)
	}

	return db
}

// Truncate empties the given tables, so each case starts from a known state.
func Truncate(t *testing.T, db *postgres.DB, tables ...string) {
	t.Helper()

	if len(tables) == 0 {
		return
	}

	statement := "TRUNCATE " + strings.Join(tables, ", ") + " CASCADE"
	if _, err := db.Pool().Exec(t.Context(), statement); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}
