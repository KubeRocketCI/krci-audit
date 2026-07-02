// Package dsn resolves the PostgreSQL connection string for the migration runner. It lets
// the same binary serve all three chart DB modes: `external` (a full DSN in a Secret) and
// `pgo`/`simple` (discrete PG* env vars wired from an operator- or chart-provisioned
// instance). The returned DSN always uses the pgx5 scheme required by golang-migrate.
package dsn

import (
	"fmt"
	"net/url"
	"os"
)

// Resolve returns a pgx5 DSN. If AUDIT_DB_DSN is set it wins (its scheme is normalized to
// pgx5). Otherwise the DSN is built from PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE
// (PGSSLMODE optional, default "disable").
func Resolve() (string, error) {
	if raw := os.Getenv("AUDIT_DB_DSN"); raw != "" {
		return normalizeScheme(raw), nil
	}

	host := os.Getenv("PGHOST")
	user := os.Getenv("PGUSER")
	db := os.Getenv("PGDATABASE")
	if host == "" || user == "" || db == "" {
		return "", fmt.Errorf("set AUDIT_DB_DSN, or PGHOST/PGUSER/PGDATABASE (+PGPASSWORD)")
	}

	port := os.Getenv("PGPORT")
	if port == "" {
		port = "5432"
	}
	sslmode := os.Getenv("PGSSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}

	return Build(host, port, user, os.Getenv("PGPASSWORD"), db, sslmode), nil
}

// Build assembles a pgx5 DSN, URL-encoding the userinfo so passwords with special
// characters are safe.
func Build(host, port, user, password, database, sslmode string) string {
	u := &url.URL{
		Scheme: "pgx5",
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   "/" + database,
	}
	if password != "" {
		u.User = url.UserPassword(user, password)
	} else {
		u.User = url.User(user)
	}
	q := url.Values{}
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}

// normalizeScheme rewrites a postgres://|postgresql:// DSN to pgx5:// so an operator-provided
// URI can be reused verbatim.
func normalizeScheme(raw string) string {
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if len(raw) >= len(prefix) && raw[:len(prefix)] == prefix {
			return "pgx5://" + raw[len(prefix):]
		}
	}
	return raw
}

// ToPostgresScheme rewrites a pgx5:// DSN to postgres://, the scheme pgx.Connect expects.
// The mirror image of normalizeScheme, for callers (like internal/migrate) that hold a
// golang-migrate DSN but need a driver connection.
func ToPostgresScheme(dsn string) string {
	const p = "pgx5://"
	if len(dsn) >= len(p) && dsn[:len(p)] == p {
		return "postgres://" + dsn[len(p):]
	}
	return dsn
}
