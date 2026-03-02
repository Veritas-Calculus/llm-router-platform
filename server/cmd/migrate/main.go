// Package main provides a CLI tool for running database migrations.
package main

import (
	"fmt"
	"log"
	"os"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/database"

	"go.uber.org/zap"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	db, err := database.New(&cfg.Database, logger)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	switch command {
	case "up":
		fmt.Println("Running migrations...")
		if err := db.Migrate(); err != nil {
			log.Fatalf("migration failed: %v", err)
		}
		fmt.Println("Migrations completed successfully.")

	case "seed":
		fmt.Println("Running migrations and seeding data...")
		if err := db.Migrate(); err != nil {
			log.Fatalf("migration failed: %v", err)
		}
		_ = db.SeedDefaultProviders()
		_ = db.SeedDefaultModels()
		_ = db.SeedDefaultAdmin(&cfg.Admin)
		fmt.Println("Migrations and seeding completed successfully.")

	case "status":
		fmt.Println("Database connection successful.")
		fmt.Printf("Host: %s:%s\n", cfg.Database.Host, cfg.Database.Port)
		fmt.Printf("Database: %s\n", cfg.Database.Name)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: migrate <command>\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  up      Run all pending migrations\n")
	fmt.Fprintf(os.Stderr, "  seed    Run migrations and seed default data\n")
	fmt.Fprintf(os.Stderr, "  status  Check database connection status\n")
}
