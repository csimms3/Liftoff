package database_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"liftoff/backend/auth"
	"liftoff/backend/database"
	"liftoff/backend/internal/testdb"
	"liftoff/backend/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/mattn/go-sqlite3"
)

const legacyOwnerID = "00000000-0000-0000-0000-000000000001"

func migrationFiles(t *testing.T) []string {
	t.Helper()
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".sql" {
			names = append(names, e.Name())
		}
	}
	return names
}

func pgCount(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestMigratePostgres_FreshDatabaseGetsFullSchema(t *testing.T) {
	pool := testdb.Postgres(t)

	for _, table := range []string{"users", "workouts", "exercises", "workout_sessions", "session_exercises",
		"exercise_sets", "dino_game_scores", "password_reset_tokens", "routines", "routine_workouts"} {
		if pgCount(t, pool, `SELECT COUNT(*) FROM information_schema.tables
			WHERE table_schema = current_schema() AND table_name = $1`, table) != 1 {
			t.Errorf("table %s missing", table)
		}
	}
	if pgCount(t, pool, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'users' AND column_name = 'is_admin'`) != 1 {
		t.Error("users.is_admin missing")
	}
	if n := pgCount(t, pool, "SELECT COUNT(*) FROM users"); n != 0 {
		t.Errorf("fresh database has %d users, want 0 (no seeded admin)", n)
	}
	if got, want := pgCount(t, pool, "SELECT COUNT(*) FROM schema_migrations"), len(migrationFiles(t)); got != want {
		t.Errorf("schema_migrations has %d rows, want %d", got, want)
	}

	// Second run is a no-op.
	if err := database.MigratePostgres(context.Background(), pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

// A database set up by the old code: 001 + 002 applied by hand, then the old
// MigratePostgres added user_id and seeded admin@liftoff.local with Admin123!.
func TestMigratePostgres_LegacyDatabaseLocksSeededAdmin(t *testing.T) {
	pool := testdb.PostgresEmpty(t)
	for _, name := range []string{"001_initial_schema.sql", "002_users.sql"} {
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		testdb.Exec(t, pool, string(body))
	}
	hash, _ := auth.HashPassword("Admin123!")
	testdb.Exec(t, pool,
		"ALTER TABLE workouts ADD COLUMN user_id VARCHAR(36)",
		"ALTER TABLE workout_sessions ADD COLUMN user_id VARCHAR(36)",
		"ALTER TABLE dino_game_scores ADD COLUMN user_id VARCHAR(36)",
		`INSERT INTO users (id, email, password_hash) VALUES ('`+legacyOwnerID+`', 'admin@liftoff.local', '`+hash+`')`,
		`INSERT INTO workouts (id, name, user_id) VALUES ('w1', 'Push', '`+legacyOwnerID+`')`,
		`INSERT INTO workouts (id, name) VALUES ('w2', 'Pre-accounts')`,
	)

	if err := database.MigratePostgres(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var gotHash string
	var isAdmin bool
	if err := pool.QueryRow(context.Background(),
		"SELECT password_hash, is_admin FROM users WHERE id = $1", legacyOwnerID).Scan(&gotHash, &isAdmin); err != nil {
		t.Fatalf("seeded admin row should be kept: %v", err)
	}
	if auth.CheckPassword("Admin123!", gotHash) || isAdmin {
		t.Errorf("seeded admin not locked: hash=%q is_admin=%v", gotHash, isAdmin)
	}
	if n := pgCount(t, pool, "SELECT COUNT(*) FROM workouts WHERE user_id = $1", legacyOwnerID); n != 2 {
		t.Errorf("legacy owner has %d workouts, want 2 (data kept, orphans assigned)", n)
	}
}

func openRawSQLite(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func sqliteExec(t *testing.T, db *sql.DB, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
}

func TestMigrateSQLite_FreshDatabase(t *testing.T) {
	db := testdb.SQLite(t).GetSQLite()

	var users, applied, isAdminCol int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&users)
	db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&applied)
	db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'is_admin'").Scan(&isAdminCol)
	if users != 0 {
		t.Errorf("fresh database has %d users, want 0 (no seeded admin)", users)
	}
	if applied != 2 {
		t.Errorf("schema_migrations has %d rows, want 2", applied)
	}
	if isAdminCol != 1 {
		t.Error("users.is_admin missing")
	}
	if err := database.MigrateSQLite(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

// A local database created by the old code: user_id present, admin seeded with
// Admin123! and owning data (the situation of the checked-in liftoff.db).
func TestMigrateSQLite_LegacyDatabaseLocksSeededAdmin(t *testing.T) {
	raw, path := openRawSQLite(t)
	hash, _ := auth.HashPassword("Admin123!")
	sqliteExec(t, raw,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE workouts (id TEXT PRIMARY KEY, name TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, user_id TEXT)`,
		`CREATE TABLE workout_sessions (id TEXT PRIMARY KEY, workout_id TEXT NOT NULL,
			started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, ended_at DATETIME,
			is_active BOOLEAN NOT NULL DEFAULT 1, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, user_id TEXT)`,
		`CREATE TABLE dino_game_scores (id TEXT PRIMARY KEY, score INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, user_id TEXT)`,
		`INSERT INTO users (id, email, password_hash) VALUES ('`+legacyOwnerID+`', 'admin@liftoff.local', '`+hash+`')`,
		`INSERT INTO users (id, email, password_hash) VALUES ('u2', 'someone@example.com', 'x')`,
		`INSERT INTO workouts (id, name, user_id) VALUES ('w1', 'Push', '`+legacyOwnerID+`')`,
		`INSERT INTO workouts (id, name, user_id) VALUES ('w2', 'Pull', 'u2')`,
	)
	raw.Close()

	db, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	s := db.GetSQLite()

	var gotHash string
	var isAdmin bool
	if err := s.QueryRow("SELECT password_hash, is_admin FROM users WHERE id = ?", legacyOwnerID).Scan(&gotHash, &isAdmin); err != nil {
		t.Fatalf("seeded admin row should be kept: %v", err)
	}
	if auth.CheckPassword("Admin123!", gotHash) || isAdmin {
		t.Errorf("seeded admin not locked: hash=%q is_admin=%v", gotHash, isAdmin)
	}
	var owner string
	s.QueryRow("SELECT user_id FROM workouts WHERE id = 'w1'").Scan(&owner)
	if owner != legacyOwnerID {
		t.Errorf("w1 owner = %q, want legacy owner (data must not be reassigned)", owner)
	}
	s.QueryRow("SELECT user_id FROM workouts WHERE id = 'w2'").Scan(&owner)
	if owner != "u2" {
		t.Errorf("w2 owner = %q, want u2", owner)
	}
}

// Rows from before user accounts existed get a locked owner, never a usable login.
func TestMigrateSQLite_PreAccountRowsGetLockedOwner(t *testing.T) {
	raw, path := openRawSQLite(t)
	sqliteExec(t, raw,
		`CREATE TABLE workouts (id TEXT PRIMARY KEY, name TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`INSERT INTO workouts (id, name) VALUES ('w1', 'Old')`,
	)
	raw.Close()

	db, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	s := db.GetSQLite()

	var owner, hash string
	s.QueryRow("SELECT user_id FROM workouts WHERE id = 'w1'").Scan(&owner)
	s.QueryRow("SELECT password_hash FROM users WHERE id = ?", legacyOwnerID).Scan(&hash)
	if owner != legacyOwnerID {
		t.Errorf("w1 owner = %q, want legacy owner", owner)
	}
	if hash != "!locked" {
		t.Errorf("legacy owner hash = %q, want locked", hash)
	}
}

// Someone registered admin@liftoff.local (random id) before the seed existed; the old
// boot code then forced Admin123! onto that row. It must be locked, and the owner
// for pre-account rows must still be created without an email collision.
func TestMigratePostgres_LocksAdminEmailWithOtherID(t *testing.T) {
	pool := testdb.PostgresEmpty(t)
	for _, name := range []string{"001_initial_schema.sql", "002_users.sql"} {
		body, _ := migrations.FS.ReadFile(name)
		testdb.Exec(t, pool, string(body))
	}
	hash, _ := auth.HashPassword("Admin123!")
	testdb.Exec(t, pool,
		`INSERT INTO users (id, email, password_hash) VALUES ('rand-id', 'Admin@liftoff.local', '`+hash+`')`,
		`INSERT INTO workouts (id, name) VALUES ('w1', 'Pre-accounts')`,
	)

	if err := database.MigratePostgres(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var got string
	pool.QueryRow(context.Background(), "SELECT password_hash FROM users WHERE id = 'rand-id'").Scan(&got)
	if auth.CheckPassword("Admin123!", got) {
		t.Error("admin@liftoff.local with a non-sentinel id still accepts Admin123!")
	}
	if n := pgCount(t, pool, "SELECT COUNT(*) FROM workouts WHERE user_id = $1", legacyOwnerID); n != 1 {
		t.Errorf("pre-account workout not assigned to legacy owner (%d)", n)
	}
}

func TestMigrateSQLite_LocksAdminEmailWithOtherID(t *testing.T) {
	raw, path := openRawSQLite(t)
	hash, _ := auth.HashPassword("Admin123!")
	sqliteExec(t, raw,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE workouts (id TEXT PRIMARY KEY, name TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`INSERT INTO users (id, email, password_hash) VALUES ('rand-id', 'admin@liftoff.local', '`+hash+`')`,
		`INSERT INTO workouts (id, name) VALUES ('w1', 'Pre-accounts')`,
	)
	raw.Close()

	db, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	s := db.GetSQLite()

	var got, owner string
	var ownerExists int
	s.QueryRow("SELECT password_hash FROM users WHERE id = 'rand-id'").Scan(&got)
	s.QueryRow("SELECT user_id FROM workouts WHERE id = 'w1'").Scan(&owner)
	s.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", legacyOwnerID).Scan(&ownerExists)
	if auth.CheckPassword("Admin123!", got) {
		t.Error("admin@liftoff.local with a non-sentinel id still accepts Admin123!")
	}
	if owner != legacyOwnerID || ownerExists != 1 {
		t.Errorf("w1 owner = %q (owner row exists: %d), want existing legacy owner", owner, ownerExists)
	}
}
