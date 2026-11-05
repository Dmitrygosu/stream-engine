// Package migrations embeds the versioned SQL schema migrations applied by goose.
package migrations

import "embed"

// FS holds the SQL migrations in goose format, ordered by their numeric prefix.
//
//go:embed *.sql
var FS embed.FS
