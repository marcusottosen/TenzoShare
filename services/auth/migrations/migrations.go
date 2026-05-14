// Package migrations embeds the SQL migration files for the auth service.
// Import this package and pass Migrations to database.RunMigrations.
package migrations

import "embed"

//go:embed *.sql
var Migrations embed.FS
