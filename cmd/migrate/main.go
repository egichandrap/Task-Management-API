package main

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"

	"task-api/internal/config"
	"task-api/internal/database/migration"

	_ "github.com/lib/pq"
)

func main() {
	cfg := config.Load()

	// Setup structured logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Parse CLI command first
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Open raw database/sql connection for migration control
	db, err := sql.Open("postgres", cfg.DBDSN)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v\nPlease check your .env configuration", err)
	}

	// Create migrator and register all migrations
	migrator := migration.NewMigrator(db, logger)
	for _, m := range migration.AllMigrations() {
		migrator.AddMigration(m)
	}

	switch command {
	case "up":
		if err := migrator.Up(); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}
	case "down":
		if err := migrator.Down(); err != nil {
			log.Fatalf("Migration down failed: %v", err)
		}
	case "reset":
		fmt.Print("WARNING: This will rollback ALL migrations. Continue? (y/N): ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Aborted.")
			return
		}
		if err := migrator.Reset(); err != nil {
			log.Fatalf("Migration reset failed: %v", err)
		}
	case "status":
		if err := migrator.Status(); err != nil {
			log.Fatalf("Migration status failed: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Task API - Database Migration Tool (PostgreSQL)")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run cmd/migrate/main.go <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  up       Apply all pending migrations")
	fmt.Println("  down     Rollback the last applied migration")
	fmt.Println("  reset    Rollback ALL migrations (with confirmation)")
	fmt.Println("  status   Show current migration status")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  Reads from .env file in project root. See .env.example for reference.")
	fmt.Println()
	fmt.Println("  DB_HOST      PostgreSQL host       (default: localhost)")
	fmt.Println("  DB_PORT      PostgreSQL port       (default: 5432)")
	fmt.Println("  DB_USER      Database user         (default: postgres)")
	fmt.Println("  DB_PASSWORD  Database password     (default: postgres)")
	fmt.Println("  DB_NAME      Database name         (default: task_api)")
	fmt.Println("  DB_SSLMODE   SSL mode              (default: disable)")
	fmt.Println("  DB_TIMEZONE  Timezone              (default: Asia/Jakarta)")
}
