package database

import (
	"context"
	"fmt"
	"log"
	"os"

	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

/**
 * Database Package
 *
 * Provides database connectivity and management for the Liftoff application.
 * Supports both PostgreSQL (primary) and SQLite (fallback) databases with
 * automatic fallback when PostgreSQL is unavailable.
 *
 * Features:
 * - Automatic database detection and connection
 * - Fallback to SQLite when PostgreSQL fails
 * - Table creation for SQLite databases
 * - Connection pooling for PostgreSQL
 * - Environment variable configuration
 */

// Database represents a database connection with support for both PostgreSQL and SQLite
type Database struct {
	pool      *pgxpool.Pool // PostgreSQL connection pool
	sqlite    *sql.DB       // SQLite database connection
	useSQLite bool          // Flag indicating which database is active
}

/**
 * NewDatabase creates a new database connection
 *
 * Attempts to connect to PostgreSQL first, then falls back to SQLite
 * if PostgreSQL is unavailable. Loads environment variables from .env file
 * if present.
 *
 * Returns:
 * - *Database: Database instance with active connection
 * - error: Connection error if both databases fail
 */
func NewDatabase() (*Database, error) {
	// Load environment variables from .env file if present
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Try PostgreSQL connection first
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		connString = "postgres://postgres:password@localhost:5432/liftoff?sslmode=disable"
	}

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		log.Println("PostgreSQL config failed, falling back to SQLite")
		return newSQLiteDatabase()
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Println("PostgreSQL connection failed, falling back to SQLite")
		return newSQLiteDatabase()
	}

	// Test the PostgreSQL connection
	if err := pool.Ping(context.Background()); err != nil {
		log.Println("PostgreSQL ping failed, falling back to SQLite")
		return newSQLiteDatabase()
	}

	log.Println("Database connected successfully (PostgreSQL)")

	return &Database{pool: pool, useSQLite: false}, nil
}

/**
 * newSQLiteDatabase creates a new SQLite database connection
 *
 * Creates the database file if it doesn't exist and initializes
 * all required tables with proper schema.
 *
 * Returns:
 * - *Database: Database instance with SQLite connection
 * - error: Connection or table creation error
 */
func newSQLiteDatabase() (*Database, error) {
	db, err := sql.Open("sqlite3", "./liftoff.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Create tables if they don't exist
	if err := createSQLiteTables(db); err != nil {
		return nil, fmt.Errorf("failed to create SQLite tables: %w", err)
	}

	log.Println("Database connected successfully (SQLite)")

	return &Database{sqlite: db, useSQLite: true}, nil
}

/**
 * createSQLiteTables initializes the SQLite database schema
 *
 * Creates all necessary tables for the workout tracking application
 * including workouts, exercises, sessions, and related data.
 *
 * Args:
 * - db: SQLite database connection
 *
 * Returns:
 * - error: Table creation error if any
 */
func createSQLiteTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS workouts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS exercises (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			sets INTEGER NOT NULL,
			reps INTEGER NOT NULL,
			weight REAL NOT NULL DEFAULT 0,
			workout_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (workout_id) REFERENCES workouts(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS workout_sessions (
			id TEXT PRIMARY KEY,
			workout_id TEXT NOT NULL,
			started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ended_at DATETIME,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (workout_id) REFERENCES workouts(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS session_exercises (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			exercise_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES workout_sessions(id) ON DELETE CASCADE,
			FOREIGN KEY (exercise_id) REFERENCES exercises(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS exercise_sets (
			id TEXT PRIMARY KEY,
			session_exercise_id TEXT NOT NULL,
			reps INTEGER NOT NULL,
			weight REAL NOT NULL,
			completed BOOLEAN NOT NULL DEFAULT 0,
			notes TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_exercise_id) REFERENCES session_exercises(id) ON DELETE CASCADE
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

func (db *Database) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
	if db.sqlite != nil {
		db.sqlite.Close()
	}
}

func (db *Database) GetPool() *pgxpool.Pool {
	return db.pool
}

func (db *Database) GetSQLite() *sql.DB {
	return db.sqlite
}

func (db *Database) IsSQLite() bool {
	return db.useSQLite
}
