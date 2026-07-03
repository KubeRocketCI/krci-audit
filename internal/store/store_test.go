package store_test

// Integration tests for the persistence contract. They apply the real embedded migrations
// to a throwaway PostgreSQL (testcontainers) and assert the required store behaviours:
// composite-PK dedup, append-only via the least-privilege writer, dry-run exclusion from the
// default read surface, partition pruning for time-bounded queries, and partition-drop
// retention.
//
// Requires Docker. If Docker is unavailable the whole test is skipped rather than failed.

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/krci-audit/internal/migrate"
	"github.com/KubeRocketCI/krci-audit/internal/models"
	"github.com/KubeRocketCI/krci-audit/internal/pgtest"
	"github.com/KubeRocketCI/krci-audit/internal/store"
)

const insertCols = `(event_uid, received_at, operation, api_group, api_version, resource, kind, namespace, name, object_uid, username, dry_run)`

// setup starts a throwaway PostgreSQL (via internal/pgtest, shared with the service
// integration tests) and returns a direct connection plus its DSN.
func setup(t *testing.T) (*pgx.Conn, string) {
	t.Helper()
	return pgtest.NewConn(t)
}

func insertEvent(t *testing.T, conn *pgx.Conn, uid string, ts time.Time, dryRun bool) int64 {
	t.Helper()
	// Ensure the monthly partition covering ts exists (install-time runway only covers the
	// current month + 2; fixed test timestamps may fall outside it).
	_, err := conn.Exec(context.Background(), `SELECT audit_ensure_partition($1)`, ts)
	require.NoError(t, err)
	tag, err := conn.Exec(context.Background(),
		`INSERT INTO audit_events `+insertCols+`
		 VALUES ($1,$2,'CREATE','v2.edp.epam.com','v1','codebases','Codebase','edp',$3,$4,'dev@example.com',$5)`,
		uid, ts, "obj-"+uid, "obj-uid-"+uid, dryRun)
	require.NoError(t, err)
	return tag.RowsAffected()
}

func TestSchemaShape(t *testing.T) {
	conn, _ := setup(t)
	ctx := context.Background()

	var partitioned bool
	require.NoError(t, conn.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_partitioned_table pt
		   JOIN pg_class c ON c.oid = pt.partrelid WHERE c.relname = $1)`, store.EventsTable).Scan(&partitioned))
	require.True(t, partitioned, "audit_events must be partitioned")

	var viewExists bool
	require.NoError(t, conn.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_views WHERE viewname = $1)`, store.RealView).Scan(&viewExists))
	require.True(t, viewExists, "audit_events_real view must exist")

	var roleExists bool
	require.NoError(t, conn.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, store.WriterRole).Scan(&roleExists))
	require.True(t, roleExists, "audit_writer role must exist")

	// Writer has INSERT/SELECT but not UPDATE/DELETE on the parent table.
	for priv, want := range map[string]bool{"INSERT": true, "SELECT": true, "UPDATE": false, "DELETE": false} {
		var has bool
		require.NoError(t, conn.QueryRow(ctx,
			`SELECT has_table_privilege($1, $2, $3)`, store.WriterRole, store.EventsTable, priv).Scan(&has))
		require.Equalf(t, want, has, "audit_writer %s privilege on audit_events", priv)
	}

	// Reader has SELECT only — strictly separate from the writer (read path cannot mutate).
	for priv, want := range map[string]bool{"SELECT": true, "INSERT": false, "UPDATE": false, "DELETE": false} {
		var has bool
		require.NoError(t, conn.QueryRow(ctx,
			`SELECT has_table_privilege($1, $2, $3)`, store.ReaderRole, store.EventsTable, priv).Scan(&has))
		require.Equalf(t, want, has, "audit_reader %s privilege on audit_events", priv)
	}
}

// The domain model's column set (internal/models) is the single source of truth; assert it
// matches the actual table so a migration change can never silently desync it.
func TestModelColumnsMatchDatabase(t *testing.T) {
	conn, _ := setup(t)
	ctx := context.Background()

	rows, err := conn.Query(ctx,
		`SELECT column_name FROM information_schema.columns WHERE table_name = $1`, store.EventsTable)
	require.NoError(t, err)
	dbCols := map[string]bool{}
	for rows.Next() {
		var c string
		require.NoError(t, rows.Scan(&c))
		dbCols[c] = true
	}
	require.NoError(t, rows.Err())

	modelCols := map[string]bool{}
	for _, c := range models.AllColumns() {
		modelCols[c] = true
		require.Truef(t, dbCols[c], "model column %q missing from audit_events table", c)
	}
	for c := range dbCols {
		require.Truef(t, modelCols[c], "table column %q missing from models.AuditEvent", c)
	}
}

// Dedup: a duplicate (event_uid, received_at) delivery is silently dropped (BEFORE INSERT
// trigger RETURN NULL) — RowsAffected 0, no error, one row stored. Spec: "Duplicate
// delivery is de-duplicated". DT covered: event identity/dedup.
func TestDedup(t *testing.T) {
	conn, _ := setup(t)
	ctx := context.Background()
	ts := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)

	require.Equal(t, int64(1), insertEvent(t, conn, "evt-dedup", ts, false), "first insert stores a row")
	require.Equal(t, int64(0), insertEvent(t, conn, "evt-dedup", ts, false), "duplicate is swallowed")

	var n int
	require.NoError(t, conn.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE event_uid = 'evt-dedup'`).Scan(&n))
	require.Equal(t, 1, n)
}

// Multiple mutations of one object are distinct events correlated by object_uid.
func TestDistinctEventsSameObject(t *testing.T) {
	conn, _ := setup(t)
	ctx := context.Background()
	base := time.Date(2026, 5, 11, 8, 0, 0, 0, time.UTC)
	_, err := conn.Exec(ctx, `SELECT audit_ensure_partition($1)`, base)
	require.NoError(t, err)

	// Same object_uid across three events, distinct event_uid + received_at.
	for i, uid := range []string{"e1", "e2", "e3"} {
		_, err := conn.Exec(ctx, `INSERT INTO audit_events `+insertCols+`
			VALUES ($1,$2,'UPDATE','v2.edp.epam.com','v1','codebases','Codebase','edp','my-svc','same-obj','dev@example.com',false)`,
			uid, base.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}
	var n int
	require.NoError(t, conn.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE object_uid = 'same-obj'`).Scan(&n))
	require.Equal(t, 3, n, "three distinct events correlated by one object_uid")
}

// Append-only: acting as the least-privilege writer, INSERT/SELECT succeed but
// UPDATE/DELETE are denied — the writer cannot mutate history.
func TestAppendOnly(t *testing.T) {
	conn, _ := setup(t)
	ctx := context.Background()
	insertEvent(t, conn, "evt-append", time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC), false)

	_, err := conn.Exec(ctx, "SET ROLE "+store.WriterRole)
	require.NoError(t, err)
	defer func() { _, _ = conn.Exec(ctx, "RESET ROLE") }()

	_, err = conn.Exec(ctx, `INSERT INTO audit_events `+insertCols+`
		VALUES ('evt-writer',now(),'CREATE','','v1','secrets','Secret','foo','a',null,'dev@example.com',false)`)
	require.NoError(t, err, "writer may INSERT")

	var n int
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM audit_events`).Scan(&n), "writer may SELECT")

	_, err = conn.Exec(ctx, `UPDATE audit_events SET name = 'tampered' WHERE event_uid = 'evt-append'`)
	require.Error(t, err, "writer must NOT UPDATE")
	require.Contains(t, strings.ToLower(err.Error()), "permission denied")

	_, err = conn.Exec(ctx, `DELETE FROM audit_events WHERE event_uid = 'evt-append'`)
	require.Error(t, err, "writer must NOT DELETE")
	require.Contains(t, strings.ToLower(err.Error()), "permission denied")
}

// Dry-run events are stored but excluded from the default read surface; an explicit
// dry_run = true predicate returns them. Spec: "Dry-run preview is recorded but hidden".
func TestDryRunExclusion(t *testing.T) {
	conn, _ := setup(t)
	ctx := context.Background()
	ts := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)

	insertEvent(t, conn, "evt-real", ts, false)
	insertEvent(t, conn, "evt-dry", ts, true)

	var realN int
	require.NoError(t, conn.QueryRow(ctx,
		`SELECT count(*) FROM audit_events_real WHERE event_uid IN ('evt-real','evt-dry')`).Scan(&realN))
	require.Equal(t, 1, realN, "default read surface excludes dry-run")

	var dryUID string
	require.NoError(t, conn.QueryRow(ctx,
		`SELECT event_uid FROM audit_events WHERE dry_run = true AND event_uid = 'evt-dry'`).Scan(&dryUID))
	require.Equal(t, "evt-dry", dryUID, "compliance query returns the preview")
}

// Time-bounded queries prune non-covering partitions. Spec: "Time-bounded query prunes
// partitions" / "Indexed query surface".
func TestPartitionPruning(t *testing.T) {
	conn, _ := setup(t)
	ctx := context.Background()

	// Guarantee two distant partitions regardless of the wall clock.
	jan := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	for _, ts := range []time.Time{jan, mar} {
		_, err := conn.Exec(ctx, `SELECT audit_ensure_partition($1)`, ts)
		require.NoError(t, err)
	}
	insertEvent(t, conn, "evt-jan", jan, false)
	insertEvent(t, conn, "evt-mar", mar, false)

	rows, err := conn.Query(ctx,
		`EXPLAIN (COSTS OFF) SELECT * FROM audit_events
		 WHERE received_at >= '2026-01-01' AND received_at < '2026-02-01'`)
	require.NoError(t, err)
	var plan strings.Builder
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		plan.WriteString(line + "\n")
	}
	require.NoError(t, rows.Err())

	require.Contains(t, plan.String(), "audit_events_202601", "January partition is scanned")
	require.NotContains(t, plan.String(), "audit_events_202603", "March partition must be pruned")
}

// Retention: dropping an expired partition removes its events as a whole-partition
// metadata operation and leaves other partitions intact and still append-only.
// Spec: "Retention drops an expired partition without row deletes".
func TestPartitionDropRetention(t *testing.T) {
	conn, _ := setup(t)
	ctx := context.Background()

	old := time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC)
	keep := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	for _, ts := range []time.Time{old, keep} {
		_, err := conn.Exec(ctx, `SELECT audit_ensure_partition($1)`, ts)
		require.NoError(t, err)
	}
	insertEvent(t, conn, "evt-old", old, false)
	insertEvent(t, conn, "evt-keep", keep, false)

	start := time.Now()
	_, err := conn.Exec(ctx, `DROP TABLE audit_events_202006`)
	require.NoError(t, err)
	elapsed := time.Since(start)
	require.Less(t, elapsed, 5*time.Second, "partition drop is a constant-time metadata op")

	var oldN, keepN int
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE event_uid = 'evt-old'`).Scan(&oldN))
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE event_uid = 'evt-keep'`).Scan(&keepN))
	require.Equal(t, 0, oldN, "dropped partition's events are gone")
	require.Equal(t, 1, keepN, "remaining partition intact")

	// Append-only still holds on the surviving partitions.
	_, err = conn.Exec(ctx, "SET ROLE "+store.WriterRole)
	require.NoError(t, err)
	defer func() { _, _ = conn.Exec(ctx, "RESET ROLE") }()
	_, err = conn.Exec(ctx, `DELETE FROM audit_events WHERE event_uid = 'evt-keep'`)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "permission denied")
}

// SetWriterPassword makes audit_writer a real LOGIN role (the deploy-time step that lets
// Vector connect in the pgo/simple/external chart modes). Verifies the writer can then log
// in and INSERT, but still cannot UPDATE — append-only holds for a genuine connection, not
// only via SET ROLE.
func TestWriterLoginPassword(t *testing.T) {
	conn, pgURL := setup(t)
	ctx := context.Background()

	pgx5 := strings.Replace(pgURL, "postgres://", "pgx5://", 1)
	require.NoError(t, migrate.SetWriterPassword(ctx, pgx5, "wr!ter p@ss#1"))

	// Build an audit_writer connection URL from the container URL.
	u, err := url.Parse(pgURL)
	require.NoError(t, err)
	u.User = url.UserPassword(store.WriterRole, "wr!ter p@ss#1")

	wconn, err := pgx.Connect(ctx, u.String())
	require.NoError(t, err, "audit_writer must be able to log in after SetWriterPassword")
	defer func() { _ = wconn.Close(ctx) }()

	_, err = wconn.Exec(ctx, `INSERT INTO audit_events `+insertCols+`
		VALUES ('evt-login',now(),'CREATE','','v1','secrets','Secret','foo','a',null,'dev@example.com',false)`)
	require.NoError(t, err, "audit_writer may INSERT")

	_, err = wconn.Exec(ctx, `UPDATE audit_events SET name = 'x' WHERE event_uid = 'evt-login'`)
	require.Error(t, err, "audit_writer must NOT UPDATE")
	require.Contains(t, strings.ToLower(err.Error()), "permission denied")

	_ = conn // container connection kept alive for the test lifetime
}

// SetReaderPassword makes audit_reader a real LOGIN role (the deploy-time step that lets the
// read API connect). Verifies the reader can then log in and SELECT, but can neither INSERT nor
// UPDATE nor DELETE — the read path is structurally non-mutating by construction.
func TestReaderLoginPassword(t *testing.T) {
	conn, pgURL := setup(t)
	ctx := context.Background()
	insertEvent(t, conn, "evt-read", time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC), false)

	pgx5 := strings.Replace(pgURL, "postgres://", "pgx5://", 1)
	require.NoError(t, migrate.SetReaderPassword(ctx, pgx5, "re@der p@ss#1"))

	u, err := url.Parse(pgURL)
	require.NoError(t, err)
	u.User = url.UserPassword(store.ReaderRole, "re@der p@ss#1")

	rconn, err := pgx.Connect(ctx, u.String())
	require.NoError(t, err, "audit_reader must be able to log in after SetReaderPassword")
	defer func() { _ = rconn.Close(ctx) }()

	var n int
	require.NoError(t, rconn.QueryRow(ctx, `SELECT count(*) FROM audit_events_real`).Scan(&n), "reader may SELECT the view")
	require.Equal(t, 1, n)

	_, err = rconn.Exec(ctx, `INSERT INTO audit_events `+insertCols+`
		VALUES ('evt-r2',now(),'CREATE','','v1','secrets','Secret','foo','a',null,'dev@example.com',false)`)
	require.Error(t, err, "audit_reader must NOT INSERT")
	require.Contains(t, strings.ToLower(err.Error()), "permission denied")

	_, err = rconn.Exec(ctx, `UPDATE audit_events SET name = 'x' WHERE event_uid = 'evt-read'`)
	require.Error(t, err, "audit_reader must NOT UPDATE")
	require.Contains(t, strings.ToLower(err.Error()), "permission denied")

	_, err = rconn.Exec(ctx, `DELETE FROM audit_events WHERE event_uid = 'evt-read'`)
	require.Error(t, err, "audit_reader must NOT DELETE")
	require.Contains(t, strings.ToLower(err.Error()), "permission denied")
}
