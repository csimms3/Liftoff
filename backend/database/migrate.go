package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"

	"liftoff/backend/auth"
	"liftoff/backend/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// legacyOwnerID owns rows that predate user accounts. Older versions seeded it as
// admin@liftoff.local with a public password; migration 006 locks it.
const legacyOwnerID = "00000000-0000-0000-0000-000000000001"

// legacyAdminEmail is the email older versions forced the public password onto.
const legacyAdminEmail = "admin@liftoff.local"

// migrationLockID is the pg_advisory_lock key that serializes concurrent migrators.
const migrationLockID = 7_311_900_001

const createSchemaMigrations = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version TEXT PRIMARY KEY,
	applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// MigratePostgres applies every embedded migrations/NNN_*.sql file that is not yet
// recorded in schema_migrations, in filename order, each in its own transaction.
// Databases created before schema_migrations existed are handled because every
// migration is idempotent (IF NOT EXISTS / conditional updates).
func MigratePostgres(ctx context.Context, pool *pgxpool.Pool) error {
	return migratePostgresFS(ctx, pool, migrations.FS)
}

func migratePostgresFS(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("take migration lock: %w", err)
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID)

	if _, err := conn.Exec(ctx, createSchemaMigrations); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied := map[string]bool{}
	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	versions, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	for _, v := range versions {
		applied[v] = true
	}

	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")
		if applied[version] {
			continue
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		// No arguments, so pgx uses the simple protocol and multi-statement files work.
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}
		log.Printf("Applied migration %s", version)
	}
	return nil
}

// sqliteMigration is the SQLite counterpart of a migrations/*.sql file. The SQL files
// use Postgres-only syntax, so SQLite gets its baseline from createSQLiteTables and
// its later changes from these steps. Each step must be safe on databases that
// already had it applied before schema_migrations existed.
type sqliteMigration struct {
	version string
	up      func(tx *sql.Tx) error
}

var sqliteMigrations = []sqliteMigration{
	{"003_user_data_isolation", sqliteUserDataIsolation},
	{"006_admin_flag", sqliteAdminFlag},
}

// MigrateSQLite applies the SQLite migration steps not yet recorded in schema_migrations.
func MigrateSQLite(db *sql.DB) error {
	if _, err := db.Exec(createSchemaMigrations); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	for _, m := range sqliteMigrations {
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", m.version).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if err := m.up(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", m.version, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.version, err)
		}
		log.Printf("Applied migration %s", m.version)
	}
	return nil
}

func sqliteHasColumn(tx *sql.Tx, table, column string) (bool, error) {
	var n int
	err := tx.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&n)
	return n > 0, err
}

// sqliteUserDataIsolation adds user_id to user-owned tables and assigns rows that
// predate user accounts to the (locked) legacy owner.
func sqliteUserDataIsolation(tx *sql.Tx) error {
	tables := []string{"workouts", "workout_sessions", "dino_game_scores"}
	orphans := false
	for _, table := range tables {
		has, err := sqliteHasColumn(tx, table, "user_id")
		if err != nil {
			return err
		}
		if !has {
			if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN user_id TEXT", table)); err != nil {
				return fmt.Errorf("add user_id to %s: %w", table, err)
			}
		}
		var n int
		if err := tx.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id IS NULL", table)).Scan(&n); err != nil {
			return err
		}
		orphans = orphans || n > 0
	}
	if !orphans {
		return nil
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash, created_at)
		VALUES (?, 'legacy-owner@liftoff.local', ?, CURRENT_TIMESTAMP)`, legacyOwnerID, auth.LockedPasswordHash); err != nil {
		return fmt.Errorf("create legacy owner: %w", err)
	}
	for _, table := range tables {
		if _, err := tx.Exec(fmt.Sprintf("UPDATE %s SET user_id = ? WHERE user_id IS NULL", table), legacyOwnerID); err != nil {
			return fmt.Errorf("assign %s to legacy owner: %w", table, err)
		}
	}
	return nil
}

// sqliteAdminFlag mirrors migrations/006_admin_flag.sql.
func sqliteAdminFlag(tx *sql.Tx) error {
	has, err := sqliteHasColumn(tx, "users", "is_admin")
	if err != nil {
		return err
	}
	if !has {
		if _, err := tx.Exec("ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT 0"); err != nil {
			return err
		}
	}
	// Match by email too: a user who registered admin@liftoff.local before the seed
	// existed had the public password forced onto their row on every boot.
	const seeded = "(id = ? OR LOWER(email) = ?)"
	if _, err := tx.Exec("DELETE FROM password_reset_tokens WHERE user_id IN (SELECT id FROM users WHERE "+seeded+")",
		legacyOwnerID, legacyAdminEmail); err != nil {
		return err
	}
	_, err = tx.Exec("UPDATE users SET password_hash = ?, is_admin = 0 WHERE "+seeded,
		auth.LockedPasswordHash, legacyOwnerID, legacyAdminEmail)
	return err
}
