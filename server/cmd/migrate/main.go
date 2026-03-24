// Package main provides a CLI tool for running database migrations.
package main

import (
	"fmt"
	"log"
	"os"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/database"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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

	db, err := database.New(&cfg.Database, cfg.Server.Mode, logger)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	switch command {
	case "up":
		fmt.Println("Running SQL migrations...")
		if err := runSQLMigrations(&cfg.Database, "up"); err != nil {
			log.Fatalf("SQL migration failed: %v", err)
		}
		fmt.Println("SQL migrations completed successfully.")

	case "down":
		steps := 1
		if len(os.Args) > 2 && os.Args[2] == "all" {
			steps = -1
		}
		fmt.Printf("Rolling back %d migration(s)...\n", steps)
		if err := runSQLMigrationsWithSteps(&cfg.Database, steps); err != nil {
			log.Fatalf("rollback failed: %v", err)
		}
		fmt.Println("Rollback completed.")

	case "auto":
		if cfg.Server.Mode == "release" {
			logger.Warn("Running AutoMigrate in release mode is NOT recommended. Please use explicit SQL migrations ('up').")
		}
		fmt.Println("Running GORM AutoMigrate (development mode)...")
		if err := db.Migrate(); err != nil {
			log.Fatalf("auto migration failed: %v", err)
		}
		fmt.Println("AutoMigrate completed.")

	case "seed":
		fmt.Println("Running migrations and seeding data...")
		if err := runSQLMigrations(&cfg.Database, "up"); err != nil {
			// Fallback to AutoMigrate if SQL migrations fail
			fmt.Println("SQL migrations failed, falling back to AutoMigrate...")
			if err := db.Migrate(); err != nil {
				log.Fatalf("migration failed: %v", err)
			}
		}
		_ = db.SeedDefaultProviders()
		_ = db.SeedDefaultModels()
		_ = db.SeedDefaultAdmin(&cfg.Admin)
		fmt.Println("Migrations and seeding completed successfully.")

	case "version":
		m, err := newMigrator(&cfg.Database)
		if err != nil {
			log.Fatalf("failed to create migrator: %v", err)
		}
		defer func() {
			if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
				log.Printf("failed to close migrator: srcErr=%v, dbErr=%v", srcErr, dbErr)
			}
		}()

		version, dirty, err := m.Version()
		if err != nil {
			fmt.Printf("Migration version: none (error: %v)\n", err)
		} else {
			fmt.Printf("Migration version: %d (dirty: %v)\n", version, dirty)
		}

	case "status":
		fmt.Println("Database connection successful.")
		fmt.Printf("Host: %s:%s\n", cfg.Database.Host, cfg.Database.Port)
		fmt.Printf("Database: %s\n", cfg.Database.Name)

		m, err := newMigrator(&cfg.Database)
		if err == nil {
			defer func() {
				if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
					log.Printf("failed to close migrator: srcErr=%v, dbErr=%v", srcErr, dbErr)
				}
			}()
			version, dirty, verr := m.Version()
			if verr == nil {
				fmt.Printf("Migration version: %d (dirty: %v)\n", version, dirty)
			} else {
				fmt.Printf("Migration version: none (%v)\n", verr)
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func newMigrator(cfg *config.DatabaseConfig) (*migrate.Migrate, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name, cfg.SSLMode,
	)

	migrationsPath := "file://migrations"

	return migrate.New(migrationsPath, dsn)
}

func runSQLMigrations(cfg *config.DatabaseConfig, direction string) error {
	m, err := newMigrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			log.Printf("failed to close migrator: srcErr=%v, dbErr=%v", srcErr, dbErr)
		}
	}()

	switch direction {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return err
		}
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return err
		}
	}
	return nil
}

func runSQLMigrationsWithSteps(cfg *config.DatabaseConfig, steps int) error {
	m, err := newMigrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			log.Printf("failed to close migrator: srcErr=%v, dbErr=%v", srcErr, dbErr)
		}
	}()

	if steps == -1 {
		return m.Down()
	}
	return m.Steps(-steps) // Negative for rollback
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: migrate <command>\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  up        Run all pending SQL migrations\n")
	fmt.Fprintf(os.Stderr, "  down      Rollback last migration (use 'down all' for everything)\n")
	fmt.Fprintf(os.Stderr, "  auto      Run GORM AutoMigrate (development only)\n")
	fmt.Fprintf(os.Stderr, "  seed      Run migrations and seed default data\n")
	fmt.Fprintf(os.Stderr, "  version   Show current migration version\n")
	fmt.Fprintf(os.Stderr, "  status    Check database connection and migration status\n")
}
