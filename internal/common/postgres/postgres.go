// Package postgres contains utilities for working with PostgreSQL.
//
// Each vertical slice owns a schema of its own and hands this package a
// [Namespace] describing it. The schemas are applied in dependency order at
// startup, from files embedded in the binary, so a deployed build carries the
// schema it expects and there is no separate artifact to keep in step.
package postgres

//nolint:revive // The migrate driver is imported for its side effect under a name that reads like a shadow.
import (
	"context"
	"database/sql"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"sort"

	"github.com/cockroachdb/errors"
	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver, for golang-migrate
	pgxUUID "github.com/vgarvardt/pgx-google-uuid/v5"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/config"
)

// Config holds PostgreSQL-related configuration.
type Config struct {
	ApplyMigrations bool   `env:"POSTGRES_MIGRATIONS_APPLY" envDefault:"true"`
	ApplySeeds      bool   `env:"POSTGRES_SEEDS_APPLY"      envDefault:"false"`
	Host            string `env:"POSTGRES_HOST"             envDefault:"localhost" validate:"required"`
	Port            string `env:"POSTGRES_PORT"             envDefault:"5432"      validate:"required"`
	Database        string `env:"POSTGRES_DB"                                      validate:"required"`
	User            string `env:"POSTGRES_USER"                                    validate:"required"`
	//nolint:gosec // The tag names an environment variable; it is not a credential.
	Password string `env:"POSTGRES_PASSWORD" validate:"required"`
	SSLMode  string `env:"POSTGRES_SSLMODE"  envDefault:"disable" validate:"oneof=disable allow prefer require verify-ca verify-full"`
}

// URL renders the configuration as a libpq connection string.
func (cfg Config) URL() string {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, cfg.Port),
		Path:     "/" + cfg.Database,
		RawQuery: url.Values{"sslmode": []string{cfg.SSLMode}}.Encode(),
	}

	return u.String()
}

// Namespace is one slice's schema, migrated as a single unit.
type Namespace struct {
	// Name is the PostgreSQL schema the slice owns, and the suffix of its
	// migration bookkeeping table.
	Name string

	// DependsOn lists namespaces that must be migrated first, for a foreign key
	// that crosses a slice boundary.
	DependsOn []string

	// Schema holds the structural migrations, under a schema/ directory.
	Schema fs.FS

	// Seed holds optional data migrations, under a seed/ directory.
	Seed fs.FS
}

// DB is the application's PostgreSQL connection pool and its migrations.
type DB struct {
	cfg        Config
	log        *slog.Logger
	pool       *pgxpool.Pool
	namespaces []*Namespace
}

type migrationType int

const (
	schema migrationType = iota
	seed
)

func (mt migrationType) string() string {
	if mt == seed {
		return "seed"
	}

	return "schema"
}

func (mt migrationType) plural() string {
	if mt == seed {
		return "seeds"
	}

	return "schemas"
}

// NewDB creates a new instance of [DB].
func NewDB(ctx context.Context, log *slog.Logger, namespaces []*Namespace) (*DB, error) {
	cfg, err := config.Load[Config]()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	orderedNamespaces, err := orderNamespaces(namespaces)
	if err != nil {
		return nil, errors.Wrap(err, "order namespaces")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.URL())
	if err != nil {
		return nil, errors.Wrap(err, "parse database url")
	}
	poolConfig.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		pgxUUID.Register(conn.TypeMap())

		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errors.Wrap(err, "connect to database")
	}

	return &DB{
		cfg:        cfg,
		log:        log,
		pool:       pool,
		namespaces: orderedNamespaces,
	}, nil
}

// orderNamespaces sorts the namespaces so that every one is preceded by the
// namespaces it depends on.
func orderNamespaces(namespaces []*Namespace) ([]*Namespace, error) {
	if len(namespaces) == 0 {
		return nil, nil
	}

	byName, inDegree, err := initializeNamespaceGraph(namespaces)
	if err != nil {
		return nil, err
	}

	dependents, err := buildNamespaceDependencies(namespaces, byName, inDegree)
	if err != nil {
		return nil, err
	}

	return topologicalSortNamespaces(byName, inDegree, dependents)
}

func initializeNamespaceGraph(namespaces []*Namespace) (map[string]*Namespace, map[string]int, error) {
	byName := make(map[string]*Namespace, len(namespaces))
	inDegree := make(map[string]int, len(namespaces))

	for _, ns := range namespaces {
		if ns == nil {
			continue
		}
		if ns.Name == "" {
			return nil, nil, errors.New("namespace with empty name")
		}
		if _, exists := byName[ns.Name]; exists {
			return nil, nil, errors.Errorf("duplicate namespace %q", ns.Name)
		}

		byName[ns.Name] = ns
		inDegree[ns.Name] = 0
	}

	return byName, inDegree, nil
}

func buildNamespaceDependencies(
	namespaces []*Namespace,
	byName map[string]*Namespace,
	inDegree map[string]int,
) (map[string][]string, error) {
	dependents := make(map[string][]string, len(namespaces))

	for _, ns := range namespaces {
		if ns == nil {
			continue
		}

		for _, dep := range ns.DependsOn {
			if dep == ns.Name {
				return nil, errors.Errorf("namespace %q cannot depend on itself", ns.Name)
			}
			if _, exists := byName[dep]; !exists {
				return nil, errors.Errorf("namespace %q depends on unknown namespace %q", ns.Name, dep)
			}

			dependents[dep] = append(dependents[dep], ns.Name)
			inDegree[ns.Name]++
		}
	}

	return dependents, nil
}

func topologicalSortNamespaces(
	byName map[string]*Namespace,
	inDegree map[string]int,
	dependents map[string][]string,
) ([]*Namespace, error) {
	queue := make([]string, 0, len(inDegree))
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}
	// Sorted at every step so that the order is the same on every start, and a
	// migration failure is reproducible rather than dependent on map ordering.
	sort.Strings(queue)

	ordered := make([]*Namespace, 0, len(inDegree))
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		ordered = append(ordered, byName[name])

		next := dependents[name]
		sort.Strings(next)
		for _, dependentName := range next {
			inDegree[dependentName]--
			if inDegree[dependentName] == 0 {
				queue = append(queue, dependentName)
			}
		}
		sort.Strings(queue)
	}

	if len(ordered) != len(inDegree) {
		return nil, errors.New("cyclic namespace dependencies detected")
	}

	return ordered, nil
}

// Pool returns the underlying [pgxpool.Pool].
func (db *DB) Pool() *pgxpool.Pool { return db.pool }

// Ping reports whether the database is reachable, which is what readiness means
// for a read-only API.
func (db *DB) Ping(ctx context.Context) error {
	return errors.WithStack(db.pool.Ping(ctx))
}

func (db *DB) applyMigration(sqlDB *sql.DB, name string, migrationFS fs.FS, dirName string) error {
	// Each namespace keeps its own bookkeeping table, so the slices' migrations
	// are numbered independently and adding one to a slice never renumbers
	// another.
	driver, err := migratepostgres.WithInstance(sqlDB, &migratepostgres.Config{
		MigrationsTable: dirName + "_migrations_" + name,
	})
	if err != nil {
		return errors.Wrap(err, "create migrate driver")
	}

	source, err := iofs.New(migrationFS, dirName)
	if err != nil {
		return errors.Wrap(err, "create migrate source")
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return errors.Wrap(err, "create migrate instance")
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return errors.Wrap(err, "apply migration")
	}

	return nil
}

func (db *DB) applyAllMigrations(ctx context.Context, addr string, mt migrationType) error {
	sqlDB, err := sql.Open("pgx", addr)
	if err != nil {
		return errors.Wrapf(err, "apply all %s: open database connection", mt.plural())
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			db.log.Error("closing db", slog.Any("err", err))
		}
	}()

	for _, ns := range db.namespaces {
		var migrationFS fs.FS
		switch mt {
		case schema:
			migrationFS = ns.Schema
		case seed:
			migrationFS = ns.Seed
		}
		if migrationFS == nil {
			continue
		}

		db.log.InfoContext(ctx, "applying "+mt.string(), slog.String("namespace", ns.Name))

		if err := db.applyMigration(sqlDB, ns.Name, migrationFS, mt.string()); err != nil {
			return errors.Wrapf(err, "applying %s for %q namespace", mt.string(), ns.Name)
		}

		db.log.InfoContext(ctx, mt.string()+" applied", slog.String("namespace", ns.Name))
	}

	return nil
}

// Migrate applies every namespace's schema, in dependency order.
func (db *DB) Migrate(ctx context.Context) error {
	return errors.WithStack(db.applyAllMigrations(ctx, db.cfg.URL(), schema))
}

// Start brings the database up: it verifies the connection and applies the
// migrations, and the seeds, if the configuration asks for them.
func (db *DB) Start(ctx context.Context) error {
	db.log.InfoContext(ctx, "starting PostgreSQL DB",
		slog.String("host", db.cfg.Host),
		slog.String("database", db.cfg.Database))

	if err := db.pool.Ping(ctx); err != nil {
		return errors.Wrap(err, "reach database")
	}

	if db.cfg.ApplyMigrations {
		if err := db.Migrate(ctx); err != nil {
			return errors.Wrap(err, "migrate")
		}
	}

	if db.cfg.ApplySeeds {
		if err := db.applyAllMigrations(ctx, db.cfg.URL(), seed); err != nil {
			return errors.Wrap(err, "seed")
		}
	}

	return nil
}

// Shutdown closes the pool.
func (db *DB) Shutdown(_ context.Context) error {
	db.pool.Close()

	return nil
}
