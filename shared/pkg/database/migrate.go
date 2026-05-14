package database

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations applies every *.sql file found in the given fs.FS in
// lexicographic (filename) order. It tracks applied migrations in a
// schema_migrations table scoped to the given schema name so each service
// only sees its own migration history.
//
// The function is idempotent: already-applied migrations are skipped.
// All SQL files must be idempotent themselves (use IF NOT EXISTS / DO NOTHING).
//
// Usage in a service main.go:
//
//	//go:embed migrations/*.sql
//	var migrationFiles embed.FS
//
//	if err := database.RunMigrations(ctx, pool, migrationFiles, "auth"); err != nil {
//	    log.Fatal("migrations failed", zap.Error(err))
//	}
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, files fs.FS, schemaName string) error {
	// Ensure the tracking table exists in the service's schema.
	createTracker := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.schema_migrations (
			name       TEXT        PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schemaName)

	if _, err := pool.Exec(ctx, createTracker); err != nil {
		return fmt.Errorf("migrations: create tracking table: %w", err)
	}

	// Collect all .sql files.
	var names []string
	if err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") {
			names = append(names, path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("migrations: walk files: %w", err)
	}
	sort.Strings(names)

	for _, name := range names {
		// Use only the base filename as the migration key so the path prefix
		// ("migrations/") doesn't matter.
		key := name
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			key = name[idx+1:]
		}

		// Skip already-applied migrations.
		var exists bool
		row := pool.QueryRow(ctx,
			fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.schema_migrations WHERE name = $1)`, schemaName),
			key,
		)
		if err := row.Scan(&exists); err != nil {
			return fmt.Errorf("migrations: check %s: %w", key, err)
		}
		if exists {
			continue
		}

		// Read and execute the SQL file.
		sql, err := fs.ReadFile(files, name)
		if err != nil {
			return fmt.Errorf("migrations: read %s: %w", name, err)
		}

		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migrations: apply %s: %w", key, err)
		}

		// Record as applied.
		if _, err := pool.Exec(ctx,
			fmt.Sprintf(`INSERT INTO %s.schema_migrations (name) VALUES ($1) ON CONFLICT DO NOTHING`, schemaName),
			key,
		); err != nil {
			return fmt.Errorf("migrations: record %s: %w", key, err)
		}
	}

	return nil
}
