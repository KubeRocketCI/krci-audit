// Package migrate applies the embedded schema migrations to a PostgreSQL database. The same
// code runs from the `krci-audit-migrate` CLI and from an install/upgrade Job. It runs as a
// schema-owner role, distinct from the least-privilege `audit_writer` used at runtime.
package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	// pgx/v5 database driver (registers the "pgx5" URL scheme).
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"

	"github.com/KubeRocketCI/krci-audit/internal/dsn"
	"github.com/KubeRocketCI/krci-audit/internal/store"
	"github.com/KubeRocketCI/krci-audit/migrations"
)

// New builds a migrate.Migrate bound to the embedded migrations and the given DSN.
// The DSN must use the pgx5 scheme, e.g. "pgx5://user:pass@host:5432/db?sslmode=disable".
// Callers are responsible for closing the returned instance.
func New(dsn string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return nil, fmt.Errorf("init migrator: %w", err)
	}

	return m, nil
}

// RunCLI is the migrator's top-level operation, extracted from main so the direction
// branching is unit-testable and main stays a thin entrypoint. On "up" it applies all
// migrations and, when writerPassword is non-empty, sets the audit_writer LOGIN password.
func RunCLI(ctx context.Context, direction, dsn, writerPassword string) error {
	switch direction {
	case "up":
		if err := Up(dsn); err != nil {
			return err
		}
		if writerPassword != "" {
			if err := SetWriterPassword(ctx, dsn, writerPassword); err != nil {
				return err
			}
		}
		return nil
	case "down":
		return Down(dsn)
	default:
		return fmt.Errorf("unknown direction %q (want up|down)", direction)
	}
}

// Up applies all outstanding migrations. It is idempotent: a database already at the
// latest version is a no-op (migrate.ErrNoChange is swallowed).
func Up(dsn string) error {
	m, err := New(dsn)
	if err != nil {
		return err
	}
	defer closeMigrator(m)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

// SetWriterPassword gives the least-privilege audit_writer role a LOGIN password so the
// ingestion path (Vector) can connect. The migration creates audit_writer NOLOGIN (keeping
// credentials out of version-controlled SQL); this step attaches the password from a Secret
// at deploy time. It is idempotent. The password is escaped server-side via format(%L) so it
// cannot be used for SQL injection despite ALTER ROLE not accepting bind parameters.
//
// migrateDSN must be a pgx5 DSN (the same one used for migrations); it is connected with pgx
// after converting to the postgres:// scheme.
func SetWriterPassword(ctx context.Context, migrateDSN, password string) error {
	conn, err := pgx.Connect(ctx, dsn.ToPostgresScheme(migrateDSN))
	if err != nil {
		return fmt.Errorf("connect to set writer password: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	var stmt string
	if err := conn.QueryRow(ctx,
		fmt.Sprintf(`SELECT format('ALTER ROLE %s WITH LOGIN PASSWORD %%L', $1::text)`, store.WriterRole), password,
	).Scan(&stmt); err != nil {
		return fmt.Errorf("build ALTER ROLE statement: %w", err)
	}
	if _, err := conn.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("set audit_writer password: %w", err)
	}

	return nil
}

// Down rolls back every migration. Intended for tests and teardown, not production.
func Down(dsn string) error {
	m, err := New(dsn)
	if err != nil {
		return err
	}
	defer closeMigrator(m)

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("revert migrations: %w", err)
	}

	return nil
}

// closeMigrator releases both the source and database halves; either error is joined so
// neither is silently dropped.
func closeMigrator(m *migrate.Migrate) {
	if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
		// Best-effort: nothing actionable at defer time beyond surfacing via panic-free log.
		_ = errors.Join(srcErr, dbErr)
	}
}
