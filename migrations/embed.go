// Package migrations embeds the versioned krci-audit SQL migrations so they can be
// applied by the migration runner (and shipped inside the migrator image) without
// depending on files present on disk at runtime.
package migrations

import "embed"

// FS holds the golang-migrate-formatted SQL files (NNNNNN_title.{up,down}.sql).
//
//go:embed *.sql
var FS embed.FS
