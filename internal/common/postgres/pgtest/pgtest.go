// Package pgtest points integration tests at a real database.
//
// A test that uses it skips when TEST_DATABASE_URL is unset, so `go test ./...`
// still works on a machine with no database; CI always sets it.
package pgtest

import (
	"context"
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

	lockDatabase(t, db)

	if err := db.Start(t.Context()); err != nil {
		t.Fatalf("migrate the test database: %v", err)
	}

	return db
}

// lockName identifies the advisory lock the integration tests serialise on.
// The value is arbitrary; only its being the same everywhere matters.
const lockName = 7_083_940_112

// lockDatabase takes an advisory lock for the duration of the test.
//
// `go test ./...` runs packages in parallel, and every slice's integration
// tests point at the same database and truncate between cases, so without
// this they empty each other's tables half-way through a run. The lock is held
// on one connection and released when the test ends.
func lockDatabase(t *testing.T, db *postgres.DB) {
	t.Helper()

	conn, err := db.Pool().Acquire(t.Context())
	if err != nil {
		t.Fatalf("acquire a connection for the test lock: %v", err)
	}

	if _, err := conn.Exec(t.Context(), "SELECT pg_advisory_lock($1)", lockName); err != nil {
		conn.Release()
		t.Fatalf("take the test lock: %v", err)
	}

	t.Cleanup(func() {
		defer conn.Release()

		if _, err := conn.Exec(context.WithoutCancel(t.Context()), "SELECT pg_advisory_unlock($1)", lockName); err != nil {
			t.Errorf("release the test lock: %v", err)
		}
	})
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
