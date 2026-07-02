// Command krci-audit-migrate applies the krci-audit schema migrations, then (on `up`)
// optionally sets the audit_writer login password from AUDIT_WRITER_PASSWORD. Configuration
// is resolved by internal/config (AUDIT_DB_DSN or PG* env), so the same image serves the
// external, pgo, and simple chart DB modes. It is idempotent and safe to run on every
// install/upgrade.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/KubeRocketCI/krci-audit/internal/config"
	"github.com/KubeRocketCI/krci-audit/internal/migrate"
)

func main() {
	log.SetFlags(0)

	dir := flag.String("direction", "up", `migration direction: "up" or "down"`)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	if err := migrate.RunCLI(context.Background(), *dir, cfg.DSN, cfg.WriterPassword); err != nil {
		log.Fatalf("migration %s failed: %v", *dir, err)
	}

	log.Printf("migration %s complete", *dir)
}
