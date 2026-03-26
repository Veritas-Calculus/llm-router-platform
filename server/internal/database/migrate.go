// Package database provides database connection and management.
// This file implements automatic SQL migration execution using golang-migrate.
package database

import (
	"fmt"
	"os"
	"path/filepath"

	"llm-router-platform/internal/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

// RunSQLMigrations applies all pending SQL migrations from the migrations
// directory. This is safe to run on every startup because golang-migrate
// tracks the current version and only applies new migrations.
//
// Migration files are searched in the following order:
//  1. ./migrations          (Docker: /app/migrations)
//  2. ../migrations         (local dev: running from server/)
//  3. server/migrations     (local dev: running from project root)
func RunSQLMigrations(cfg *config.DatabaseConfig, logger *zap.Logger) error {
	migrationsDir := findMigrationsDir()
	if migrationsDir == "" {
		logger.Info("no migrations directory found, skipping SQL migrations")
		return nil
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name, cfg.SSLMode,
	)

	absPath, _ := filepath.Abs(migrationsDir)
	sourceURL := "file://" + absPath

	m, err := migrate.New(sourceURL, dsn)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			logger.Warn("migrator source close error", zap.Error(srcErr))
		}
		if dbErr != nil {
			logger.Warn("migrator db close error", zap.Error(dbErr))
		}
	}()

	version, dirty, verr := m.Version()
	if verr != nil && verr != migrate.ErrNoChange {
		logger.Info("migration state", zap.String("status", "no migrations applied yet"))
	} else {
		logger.Info("current migration version", zap.Uint("version", version), zap.Bool("dirty", dirty))
	}

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			logger.Info("SQL migrations: already up-to-date")
			return nil
		}
		return fmt.Errorf("migration failed: %w", err)
	}

	newVersion, _, _ := m.Version()
	logger.Info("SQL migrations applied successfully", zap.Uint("new_version", newVersion))
	return nil
}

// findMigrationsDir checks common locations for the migrations directory.
func findMigrationsDir() string {
	candidates := []string{
		"migrations",
		"../migrations",
		"server/migrations",
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}
