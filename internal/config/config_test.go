package config

import "testing"

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "postgres://u:p@h:5432/d?sslmode=disable")
	t.Setenv("AUDIT_WRITER_PASSWORD", "wp")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DSN != "pgx5://u:p@h:5432/d?sslmode=disable" {
		t.Fatalf("DSN = %q", c.DSN)
	}
	if c.WriterPassword != "wp" {
		t.Fatalf("WriterPassword = %q", c.WriterPassword)
	}
}

func TestLoadMissingDB(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "")
	t.Setenv("PGHOST", "")
	t.Setenv("PGUSER", "")
	t.Setenv("PGDATABASE", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when the database cannot be resolved")
	}
}
