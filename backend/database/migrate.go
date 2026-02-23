package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

const adminUserID = "00000000-0000-0000-0000-000000000001"
const adminEmail = "admin@liftoff.local"
const adminPasswordHash = "$2a$10$SlmOtj3A17j2JLju8e9VfeHZo/SjwuC4ciN0mbSXR9gILDiuaJexe"

// MigrateSQLite runs pending migrations on SQLite (adds user_id, migrates data)
func MigrateSQLite(db *sql.DB) error {
	// Check if workouts has user_id column
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('workouts') WHERE name='user_id'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check schema: %w", err)
	}
	if count > 0 {
		return nil // Already migrated
	}

	log.Println("Running migration: add user_id to workouts, sessions, dino_game_scores")

	// Add user_id columns
	for _, table := range []string{"workouts", "workout_sessions", "dino_game_scores"} {
		_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN user_id TEXT", table))
		if err != nil {
			return fmt.Errorf("failed to add user_id to %s: %w", table, err)
		}
	}

	// Create admin user for migrated data (password: Admin123!)
	_, err = db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		adminUserID, adminEmail, adminPasswordHash)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	// Migrate existing data to admin user
	for _, table := range []string{"workouts", "workout_sessions", "dino_game_scores"} {
		_, err = db.Exec(fmt.Sprintf("UPDATE %s SET user_id = ? WHERE user_id IS NULL", table), adminUserID)
		if err != nil {
			return fmt.Errorf("failed to migrate %s: %w", table, err)
		}
	}

	log.Println("Migration completed: existing data assigned to admin@liftoff.local (password: Admin123!)")
	return nil
}

// MigratePostgres runs pending migrations on PostgreSQL
func MigratePostgres(pool *pgxpool.Pool) error {
	ctx := context.Background()

	// Check if workouts has user_id
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'workouts' AND column_name = 'user_id'
		)`).Scan(&exists)
	if err != nil || exists {
		return err
	}

	log.Println("Running migration: add user_id to workouts, sessions, dino_game_scores")

	// Add columns
	for _, alter := range []string{
		"ALTER TABLE workouts ADD COLUMN user_id VARCHAR(36)",
		"ALTER TABLE workout_sessions ADD COLUMN user_id VARCHAR(36)",
		"ALTER TABLE dino_game_scores ADD COLUMN user_id VARCHAR(36)",
	} {
		_, err = pool.Exec(ctx, alter)
		if err != nil {
			return fmt.Errorf("failed to add column: %w", err)
		}
	}

	// Create admin user
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (email) DO NOTHING`, adminUserID, adminEmail, adminPasswordHash)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	// Migrate data
	for _, table := range []string{"workouts", "workout_sessions", "dino_game_scores"} {
		_, err = pool.Exec(ctx, fmt.Sprintf("UPDATE %s SET user_id = $1 WHERE user_id IS NULL", table), adminUserID)
		if err != nil {
			return fmt.Errorf("failed to migrate %s: %w", table, err)
		}
	}

	// Add NOT NULL and indexes
	for _, stmt := range []string{
		"ALTER TABLE workouts ALTER COLUMN user_id SET NOT NULL",
		"ALTER TABLE workout_sessions ALTER COLUMN user_id SET NOT NULL",
		"ALTER TABLE dino_game_scores ALTER COLUMN user_id SET NOT NULL",
		"CREATE INDEX IF NOT EXISTS idx_workouts_user_id ON workouts(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_workout_sessions_user_id ON workout_sessions(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_dino_game_scores_user_id ON dino_game_scores(user_id)",
	} {
		_, err = pool.Exec(ctx, stmt)
		if err != nil {
			return fmt.Errorf("failed to finalize migration: %w", err)
		}
	}

	log.Println("Migration completed: existing data assigned to admin@liftoff.local (password: Admin123!)")
	return nil
}
