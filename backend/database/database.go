package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/mattn/go-sqlite3"
)

// Package database connects to PostgreSQL or SQLite and keeps the schema current.
//
// With DATABASE_URL set (production), only that Postgres is used. If it is
// unreachable at startup the server still starts, reports not-ready (the API
// answers 503) and keeps retrying in the background; it never falls back to
// SQLite, whose data would silently vanish on an ephemeral disk.
//
// Without DATABASE_URL (local dev), a Postgres on localhost is used if one is
// running, otherwise SQLite in ./liftoff.db.

// localDevURL is the Postgres tried in local dev when DATABASE_URL is unset.
const localDevURL = "postgres://postgres:password@localhost:5432/liftoff?sslmode=disable"

// Retry backoff while the configured Postgres is unreachable, and how often a
// connected Postgres is pinged so an outage after startup is noticed too.
const (
	retryMinDelay  = 2 * time.Second
	retryMaxDelay  = 30 * time.Second
	healthInterval = 5 * time.Second
	pingTimeout    = 3 * time.Second
)

// Database represents a database connection with support for both PostgreSQL and SQLite
type Database struct {
	pool      *pgxpool.Pool // PostgreSQL connection pool
	sqlite    *sql.DB       // SQLite database connection
	useSQLite bool          // Flag indicating which database is active

	ready atomic.Bool        // reachable and migrated
	stop  context.CancelFunc // stops the background monitor
}

// NewDatabase opens the database described by the environment (see package doc).
// It fails for configuration errors and failed migrations, never because
// Postgres is down.
func NewDatabase() (*Database, error) {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return OpenPostgres(url)
	}
	return newLocalDatabase()
}

// OpenPostgres opens the Postgres at url. If it can't be reached now, the returned
// Database is not Ready and connects in the background. A migration that fails
// against a reachable database is an error: retrying won't fix it, and a server
// that looks alive but never serves would hide a bad deploy.
func OpenPostgres(url string) (*Database, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}
	// Connections are opened lazily, so this doesn't need the server to be up.
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}

	ctx, stop := context.WithCancel(context.Background())
	db := &Database{pool: pool, stop: stop}
	ping := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, pingTimeout)
		defer cancel()
		return pool.Ping(ctx)
	}
	migrate := func(ctx context.Context) error { return MigratePostgres(ctx, pool) }

	migrated := false
	if err := ping(ctx); err != nil {
		log.Printf("PostgreSQL unavailable, serving 503 and retrying in the background: %v", err)
	} else {
		if err := migrate(ctx); err != nil {
			stop()
			pool.Close()
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		migrated = true
		db.ready.Store(true)
		log.Println("Database connected successfully (PostgreSQL)")
	}
	go db.monitor(ctx, ping, migrate, migrated, monitorTiming{retryMinDelay, retryMaxDelay, healthInterval}, fatalf)
	return db, nil
}

type monitorTiming struct{ minDelay, maxDelay, healthEvery time.Duration }

// fatalf ends the process; a variable so tests can observe it.
var fatalf = log.Fatalf

// monitor keeps ready accurate for the life of the server. While the database is
// unreachable it retries with exponential backoff; once reachable it migrates
// (first time only) and marks ready; after that it pings every healthEvery and
// drops back to not-ready after two failed pings in a row (one slow ping, e.g.
// every pooled connection busy, isn't an outage). A migration failure is fatal.
func (db *Database) monitor(ctx context.Context, ping, migrate func(context.Context) error, migrated bool, t monitorTiming, fatal func(string, ...any)) {
	delay := t.minDelay
	failures := 0
	for {
		wait := delay
		if db.ready.Load() && failures == 0 {
			wait = t.healthEvery // after one failed ping, recheck sooner
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		if err := ping(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			failures++
			if db.ready.Load() && failures < 2 {
				continue
			}
			if db.ready.Swap(false) {
				log.Printf("PostgreSQL connection lost, serving 503 until it is back: %v", err)
				delay = t.minDelay
			} else {
				delay = min(delay*2, t.maxDelay)
				log.Printf("PostgreSQL still unavailable (next try in %s): %v", delay, err)
			}
			continue
		}
		if !migrated {
			if err := migrate(ctx); err != nil {
				fatal("failed to run migrations: %v", err)
				return
			}
			migrated = true
		}
		if !db.ready.Swap(true) {
			log.Println("Database connected successfully (PostgreSQL)")
		}
		failures = 0
		delay = t.minDelay
	}
}

// newLocalDatabase is the local-dev path: a local Postgres if running, else SQLite.
func newLocalDatabase() (*Database, error) {
	config, err := pgxpool.ParseConfig(localDevURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err == nil {
		err = pool.Ping(context.Background())
	}
	if err != nil {
		if pool != nil {
			pool.Close()
		}
		log.Println("No local PostgreSQL and DATABASE_URL unset: using SQLite (./liftoff.db)")
		return newSQLiteDatabase()
	}
	if err := MigratePostgres(context.Background(), pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	log.Println("Database connected successfully (local PostgreSQL)")
	db := &Database{pool: pool}
	db.ready.Store(true)
	return db, nil
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
	return OpenSQLite("./liftoff.db")
}

// OpenSQLite opens (creating if needed) the SQLite database at path and brings its
// schema up to date.
func OpenSQLite(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Create tables if they don't exist
	if err := createSQLiteTables(db); err != nil {
		return nil, fmt.Errorf("failed to create SQLite tables: %w", err)
	}

	if err := MigrateSQLite(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database connected successfully (SQLite)")

	d := &Database{sqlite: db, useSQLite: true}
	d.ready.Store(true)
	return d, nil
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
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(LOWER(email))`,
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
		`CREATE TABLE IF NOT EXISTS dino_game_scores (
			id TEXT PRIMARY KEY,
			score INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS routines (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_routines_user_id ON routines(user_id)`,
		`CREATE TABLE IF NOT EXISTS routine_workouts (
			id TEXT PRIMARY KEY,
			routine_id TEXT NOT NULL REFERENCES routines(id) ON DELETE CASCADE,
			workout_id TEXT NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
			slot_order INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_routine_workouts_routine_id ON routine_workouts(routine_id)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

// Ready reports whether the database is connected and migrated. Until it is,
// the API answers 503.
func (db *Database) Ready() bool {
	return db.ready.Load()
}

func (db *Database) Close() {
	if db.stop != nil {
		db.stop()
	}
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
