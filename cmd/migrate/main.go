package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/example/pets/internal/config"
	"github.com/example/pets/internal/db"
)

func setupLogger(cfg *config.Config) {
	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if strings.ToLower(cfg.LogFormat) == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func main() {
	cfg := config.Load()
	setupLogger(cfg)

	flag.Parse()
	command := flag.Arg(0)
	if command == "" {
		command = "up"
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	switch command {
	case "up":
		slog.Info("Applying database migrations...")
		if err := db.Migrate(ctx, pool); err != nil {
			slog.Error("Migration failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Migrations applied successfully")
	case "down":
		slog.Info("Rolling back last migration...")
		if err := db.MigrateDown(ctx, pool); err != nil {
			slog.Error("Rollback failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Rollback completed successfully")
	case "status":
		statuses, err := db.MigrationStatus(ctx, pool)
		if err != nil {
			slog.Error("Failed to get migration status", "error", err)
			os.Exit(1)
		}
		fmt.Printf("%-20s %-30s %s\n", "VERSION", "MIGRATION", "STATUS")
		for _, s := range statuses {
			statusStr := "Pending"
			if s.State == "applied" {
				statusStr = fmt.Sprintf("Applied at %s", s.AppliedAt.Format("2006-01-02 15:04:05"))
			}
			fmt.Printf("%-20d %-30s %s\n", s.Source.Version, s.Source.Path, statusStr)
		}
	default:
		fmt.Fprintf(os.Stderr, "Usage: migrate [up|down|status]\n")
		os.Exit(1)
	}
}
