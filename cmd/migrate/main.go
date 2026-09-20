package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/example/pets/internal/config"
	"github.com/example/pets/internal/db"
)

func main() {
	cfg := config.Load()

	flag.Parse()
	command := flag.Arg(0)
	if command == "" {
		command = "up"
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	switch command {
	case "up":
		log.Println("Applying database migrations...")
		if err := db.Migrate(ctx, pool); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Println("Migrations applied successfully.")
	case "down":
		log.Println("Rolling back last migration...")
		if err := db.MigrateDown(ctx, pool); err != nil {
			log.Fatalf("Rollback failed: %v", err)
		}
		log.Println("Rollback completed successfully.")
	case "status":
		statuses, err := db.MigrationStatus(ctx, pool)
		if err != nil {
			log.Fatalf("Failed to get migration status: %v", err)
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
