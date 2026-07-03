// Package pgtest is a shared test harness that starts a throwaway PostgreSQL (testcontainers),
// applies the real embedded migrations, and returns a connected pgxpool. The read API's
// service integration tests (internal/services/*) all read the same store, so this factors the
// container + migration boilerplate out of each package rather than duplicating it. If Docker
// is unavailable the test is skipped, mirroring the store package's own integration tests.
package pgtest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/KubeRocketCI/krci-audit/internal/migrate"
)

// newDSN starts a PostgreSQL container, applies all migrations, and returns its connection
// DSN. The container is torn down via t.Cleanup. The test is skipped (not failed) when Docker
// is unavailable. This is the single bootstrap every helper in this package builds on, so the
// container image, wait strategy, and Docker-unavailable detection live in exactly one place.
func newDSN(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("audit"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("Docker unavailable, skipping integration tests: %v", err)
		}
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	pgURL, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	require.NoError(t, migrate.Up(strings.Replace(pgURL, "postgres://", "pgx5://", 1)))

	return pgURL
}

// New starts a PostgreSQL container, applies all migrations, and returns a pgxpool.Pool bound
// to it. The pool is closed via t.Cleanup.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pgURL := newDSN(t)

	pool, err := pgxpool.New(context.Background(), pgURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

// NewConn is like New but returns a single pgx.Conn plus the raw connection DSN, for tests
// that need direct connection control (e.g. SET ROLE, or building a second connection under a
// different role/password).
func NewConn(t *testing.T) (*pgx.Conn, string) {
	t.Helper()
	pgURL := newDSN(t)

	conn, err := pgx.Connect(context.Background(), pgURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	return conn, pgURL
}

func isDockerUnavailable(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cannot connect to the docker daemon") ||
		strings.Contains(msg, "docker daemon") ||
		strings.Contains(msg, "rootless docker not found") ||
		strings.Contains(msg, "failed to find a working docker")
}

// Ptr returns a pointer to v — a small convenience for building the optional (*string, *bool,
// ...) fields on Filter/SeedEvent fixtures across the service test suites.
func Ptr[T any](v T) *T { return &v }

// SeedInsertCols is the column list for the InsertEvent helper; exported so callers can build
// matching VALUES if they insert directly.
const SeedInsertCols = `(event_uid, received_at, operation, api_group, api_version, resource, kind, sub_resource, namespace, name, object_uid, username, dry_run)`

// InsertEvent inserts a single audit event, first ensuring the monthly partition covering ts
// exists (install-time runway covers only the current month + 2). It is the seeding primitive
// for the service integration tests.
func InsertEvent(t *testing.T, pool *pgxpool.Pool, e SeedEvent) {
	t.Helper()
	ctx := context.Background()

	_, err := pool.Exec(ctx, `SELECT audit_ensure_partition($1)`, e.ReceivedAt)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO audit_events `+SeedInsertCols+`
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		e.EventUID, e.ReceivedAt, e.Operation, e.APIGroup, e.APIVersion, e.Resource,
		e.Kind, e.SubResource, e.Namespace, e.Name, e.ObjectUID, e.Username, e.DryRun)
	require.NoError(t, err)
}

// SeedEvent is a convenience shape for InsertEvent.
type SeedEvent struct {
	EventUID    string
	ReceivedAt  time.Time
	Operation   string
	APIGroup    string
	APIVersion  string
	Resource    string
	Kind        string
	SubResource *string
	Namespace   string
	Name        string
	ObjectUID   *string
	Username    string
	DryRun      bool
}
