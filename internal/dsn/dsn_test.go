package dsn

import (
	"net/url"
	"testing"
)

func TestBuildEncodesPassword(t *testing.T) {
	got := Build("db-host", "5432", "audit_writer", "p@ss w/ord:#", "krci-audit", "require")

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("Build produced an unparseable URL %q: %v", got, err)
	}
	if u.Scheme != "pgx5" {
		t.Fatalf("scheme = %q, want pgx5", u.Scheme)
	}
	if u.Host != "db-host:5432" {
		t.Fatalf("host = %q", u.Host)
	}
	if u.Path != "/krci-audit" {
		t.Fatalf("path = %q", u.Path)
	}
	pw, _ := u.User.Password()
	if pw != "p@ss w/ord:#" {
		t.Fatalf("password round-trip failed: %q", pw)
	}
	if u.Query().Get("sslmode") != "require" {
		t.Fatalf("sslmode = %q", u.Query().Get("sslmode"))
	}
}

func TestResolvePrefersExplicitDSN(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "postgres://u:p@h:5432/d?sslmode=disable")
	got, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got != "pgx5://u:p@h:5432/d?sslmode=disable" {
		t.Fatalf("Resolve normalized scheme wrong: %q", got)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	want := "pgx5://krci-audit:secret@krci-audit-primary:5432/krci-audit?sslmode=disable"
	if got != want {
		t.Fatalf("Resolve = %q, want %q (default port/sslmode)", got, want)
	}
}

func TestResolveMissing(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "")
	t.Setenv("PGHOST", "")
	t.Setenv("PGUSER", "")
	t.Setenv("PGDATABASE", "")
	if _, err := Resolve(); err == nil {
		t.Fatal("expected error when neither DSN nor PG* env is set")
	}
}
