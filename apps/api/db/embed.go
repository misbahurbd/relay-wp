// Package db embeds the SQL migrations so the binary can run them without files on disk.
package db

import "embed"

// Migrations holds every goose migration in migrations/.
//
//go:embed migrations/*.sql
var Migrations embed.FS
