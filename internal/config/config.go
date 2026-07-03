// Package config aggregates the migrator's runtime configuration in one typed place. It is
// the single point where env-sourced inputs are read and validated, rather than scattering
// os.Getenv calls across entrypoints.
package config

import (
	"os"

	"github.com/KubeRocketCI/krci-audit/internal/dsn"
)

// Config holds the resolved inputs for the migration runner.
type Config struct {
	// DSN is the pgx5 connection string for the schema-owner role.
	DSN string
	// WriterPassword, when non-empty, is applied as the audit_writer LOGIN password after
	// migrations run (the deploy-time step that lets Vector connect). Sourced from
	// AUDIT_WRITER_PASSWORD.
	WriterPassword string
	// ReaderPassword, when non-empty, is applied as the audit_reader LOGIN password after
	// migrations run (the deploy-time step that lets the read API connect). Sourced from
	// AUDIT_READER_PASSWORD.
	ReaderPassword string
}

// Load resolves configuration from the environment (AUDIT_DB_DSN or the discrete PG* vars,
// plus AUDIT_WRITER_PASSWORD). It returns an error if the database connection cannot be
// resolved.
func Load() (*Config, error) {
	d, err := dsn.Resolve()
	if err != nil {
		return nil, err
	}

	return &Config{
		DSN:            d,
		WriterPassword: os.Getenv("AUDIT_WRITER_PASSWORD"),
		ReaderPassword: os.Getenv("AUDIT_READER_PASSWORD"),
	}, nil
}

// APIConfig holds the resolved inputs for the read API server. It is deliberately separate
// from Config (the migrator's owner-role config): the API connects as the least-privilege
// audit_reader role, so the two entrypoints never share credentials.
type APIConfig struct {
	// DSN is the pgx5 connection string for the audit_reader role.
	DSN string
	// Port is the HTTP listen port (default 8080).
	Port string
}

// LoadAPI resolves the read API's configuration from the environment. The reader DSN is
// sourced from AUDIT_DB_DSN or the discrete PG* vars (same resolution as the migrator, but
// the deployment supplies the audit_reader credentials); PORT defaults to 8080.
func LoadAPI() (*APIConfig, error) {
	d, err := dsn.Resolve()
	if err != nil {
		return nil, err
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &APIConfig{DSN: d, Port: port}, nil
}
