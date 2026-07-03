package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "postgres://u:p@h:5432/d?sslmode=disable")
	t.Setenv("AUDIT_WRITER_PASSWORD", "wp")

	c, err := Load()
	require.NoError(t, err)
	require.Equal(t, "pgx5://u:p@h:5432/d?sslmode=disable", c.DSN)
	require.Equal(t, "wp", c.WriterPassword)
}

func TestLoadMissingDB(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "")
	t.Setenv("PGHOST", "")
	t.Setenv("PGUSER", "")
	t.Setenv("PGDATABASE", "")

	_, err := Load()
	require.Error(t, err, "expected error when the database cannot be resolved")
}

func TestLoadAPIFromEnv(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "postgres://u:p@h:5432/d?sslmode=disable")
	t.Setenv("PORT", "9090")

	c, err := LoadAPI()
	require.NoError(t, err)
	require.Equal(t, "pgx5://u:p@h:5432/d?sslmode=disable", c.DSN)
	require.Equal(t, "9090", c.Port)
}

func TestLoadAPIDefaultsPort(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "postgres://u:p@h:5432/d?sslmode=disable")
	t.Setenv("PORT", "")

	c, err := LoadAPI()
	require.NoError(t, err)
	require.Equal(t, "8080", c.Port, "Port must default to 8080")
}

func TestLoadAPIMissingDB(t *testing.T) {
	t.Setenv("AUDIT_DB_DSN", "")
	t.Setenv("PGHOST", "")
	t.Setenv("PGUSER", "")
	t.Setenv("PGDATABASE", "")

	_, err := LoadAPI()
	require.Error(t, err, "expected error when the database cannot be resolved")
}
