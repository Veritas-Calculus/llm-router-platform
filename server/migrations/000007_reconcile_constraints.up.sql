-- Migration 000007: Reconcile constraint names to match GORM AutoMigrate expectations
-- This migration renames SQL-defined constraints to match the naming convention
-- that GORM uses when running AutoMigrate, preventing "constraint does not exist"
-- errors when both SQL migrations and GORM AutoMigrate are used together.

-- Fix dlp_configs unique constraint name
ALTER TABLE dlp_configs RENAME CONSTRAINT idx_dlp_configs_project TO uni_dlp_configs_project_id;
