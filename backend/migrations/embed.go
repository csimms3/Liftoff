// Package migrations embeds the PostgreSQL schema migrations so the server
// binary can apply them on startup.
package migrations

import "embed"

// FS holds the ordered NNN_name.sql migration files.
//
//go:embed *.sql
var FS embed.FS
