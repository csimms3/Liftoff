package database_test

import (
	"context"
	"path/filepath"
	"testing"

	"liftoff/backend/auth"
	"liftoff/backend/database"
	"liftoff/backend/internal/testdb"
	"liftoff/backend/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
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

func TestMigrate_FreshDatabaseGetsFullSchema(t *testing.T) {
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
	if err := database.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

// A database set up by the old code: 001 + 002 applied by hand, then the old
// startup code added user_id and seeded admin@liftoff.local with Admin123!.
func TestMigrate_LegacyDatabaseLocksSeededAdmin(t *testing.T) {
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

	if err := database.Migrate(context.Background(), pool); err != nil {
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

// Someone registered admin@liftoff.local (random id) before the seed existed; the old
// boot code then forced Admin123! onto that row. It must be locked, and the owner
// for pre-account rows must still be created without an email collision.
func TestMigrate_LocksAdminEmailWithOtherID(t *testing.T) {
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

	if err := database.Migrate(context.Background(), pool); err != nil {
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
