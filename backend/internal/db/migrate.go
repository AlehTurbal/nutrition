package db

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate applies any embedded SQL migrations that have not yet been run. Each
// file is applied in lexical order inside its own transaction and recorded in
// the schema_migrations table, so Migrate is safe to run on every startup.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		applied, err := isApplied(ctx, pool, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, pool, name); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}

	return nil
}

func isApplied(ctx context.Context, pool *pgxpool.Pool, name string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, name,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}
	return exists, nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, name string) error {
	sqlBytes, err := migrationFS.ReadFile("migrations/" + name)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`, name,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
