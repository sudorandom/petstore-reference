package db

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/example/pets/sql/schema"
)

// Migrate applies all pending database migrations against the provided connection pool.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() {
		_ = sqlDB.Close()
	}()

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, schema.FS)
	if err != nil {
		return fmt.Errorf("failed to create goose provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	for _, res := range results {
		log.Printf("Applied migration %d: %s (%s)", res.Source.Version, res.Source.Path, res.Duration)
	}

	return nil
}

// MigrateDown rolls back the most recent database migration.
func MigrateDown(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() {
		_ = sqlDB.Close()
	}()

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, schema.FS)
	if err != nil {
		return fmt.Errorf("failed to create goose provider: %w", err)
	}

	res, err := provider.Down(ctx)
	if err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	if res != nil {
		log.Printf("Rolled back migration %d: %s (%s)", res.Source.Version, res.Source.Path, res.Duration)
	}

	return nil
}

// MigrationStatus returns the status of all available migrations.
func MigrationStatus(ctx context.Context, pool *pgxpool.Pool) ([]*goose.MigrationStatus, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() {
		_ = sqlDB.Close()
	}()

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, schema.FS)
	if err != nil {
		return nil, fmt.Errorf("failed to create goose provider: %w", err)
	}

	return provider.Status(ctx)
}
