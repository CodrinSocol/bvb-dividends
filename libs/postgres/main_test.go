package postgres_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/postgres"
)

// testPool is the shared connection for the integration tests, or nil when no
// database was configured.
var testPool *pgxpool.Pool

// TestMain connects to the database named by TEST_DATABASE_URL and applies the
// migrations once for the whole package.
//
// Without that variable the integration tests skip rather than fail, so
// `go test ./...` still works on a machine with no database. CI always sets it.
func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "TEST_DATABASE_URL is not set; skipping postgres integration tests")
		os.Exit(m.Run())
	}

	// These tests TRUNCATE between cases, so pointing them at a database
	// someone is using would silently destroy its contents. Requiring the name
	// to say it is a test database makes that mistake impossible rather than
	// merely unlikely; the author of this file already made it once.
	if !strings.Contains(databaseName(url), "test") {
		fmt.Fprintf(os.Stderr,
			"refusing to run destructive tests against %q: TEST_DATABASE_URL must name a database with \"test\" in it\n",
			databaseName(url))
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := postgres.Connect(ctx, url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to test database: %v\n", err)
		os.Exit(1)
	}
	if err := postgres.Migrate(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "migrate test database: %v\n", err)
		os.Exit(1)
	}

	testPool = pool
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

// newTestPool returns the shared pool with the tables emptied, or skips the
// test when no database is configured.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	// dividend cascades from company, so one truncate clears both.
	if _, err := testPool.Exec(context.Background(), "TRUNCATE company CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return testPool
}

// databaseName extracts the database from a connection URL, returning "" when
// it cannot be read; an unreadable name fails the guard above, which is the
// safe direction.
func databaseName(connectionURL string) string {
	parsed, err := url.Parse(connectionURL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsed.Path, "/")
}
