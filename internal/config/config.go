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
	}, nil
}
