package dsn

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildEncodesPassword(t *testing.T) {
	got := Build("db-host", "5432", "audit_writer", "p@ss w/ord:#", "krci-audit", "require")

	u, err := url.Parse(got)
	require.NoError(t, err, "Build produced an unparseable URL %q", got)
	require.Equal(t, "pgx5", u.Scheme)
	require.Equal(t, "db-host:5432", u.Host)
	require.Equal(t, "/krci-audit", u.Path)

	pw, _ := u.User.Password()
	require.Equal(t, "p@ss w/ord:#", pw, "password round-trip failed")
	require.Equal(t, "require", u.Query().Get("sslmode"))
}

func TestResolvePrefersExplicitDSN(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "postgres://u:p@h:5432/d?sslmode=disable")

	got, err := Resolve()
	require.NoError(t, err)
	require.Equal(t, "pgx5://u:p@h:5432/d?sslmode=disable", got, "Resolve normalized scheme wrong")
}

func TestResolveFromDiscreteEnv(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "")
	t.Setenv("PGHOST", "krci-audit-primary")
	t.Setenv("PGUSER", "krci-audit")
	t.Setenv("PGPASSWORD", "secret")
	t.Setenv("PGDATABASE", "krci-audit")
	t.Setenv("PGPORT", "")
	t.Setenv("PGSSLMODE", "")

	got, err := Resolve()
	require.NoError(t, err)
	require.Equal(t, "pgx5://krci-audit:secret@krci-audit-primary:5432/krci-audit?sslmode=disable", got,
		"Resolve default port/sslmode")
}

func TestToPostgresScheme(t *testing.T) {
	got := ToPostgresScheme("pgx5://u:p@h:5432/d?sslmode=disable")
	require.Equal(t, "postgres://u:p@h:5432/d?sslmode=disable", got)
}

func TestToPostgresSchemeLeavesOtherSchemesAlone(t *testing.T) {
	got := ToPostgresScheme("postgres://u:p@h:5432/d")
	require.Equal(t, "postgres://u:p@h:5432/d", got, "ToPostgresScheme must not touch a non-pgx5 DSN")
}

func TestResolveMissing(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "")
	t.Setenv("PGHOST", "")
	t.Setenv("PGUSER", "")
	t.Setenv("PGDATABASE", "")

	_, err := Resolve()
	require.Error(t, err, "expected error when neither DSN nor PG* env is set")
}
