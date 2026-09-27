package database_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

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

// upTo returns the embedded migrations up to and including version.
func upTo(t *testing.T, version string) fstest.MapFS {
	t.Helper()
	fsys := fstest.MapFS{}
	for _, name := range migrationFiles(t) {
		if strings.TrimSuffix(name, ".sql") > version {
			continue
		}
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		fsys[name] = &fstest.MapFile{Data: body}
	}
	return fsys
}

// 008 on a database with history: same-named exercises (any case/spacing) in
// different workouts become one movement per user, sessions get their names
// copied, and deleting a workout afterwards keeps its history.
func TestMigrate_MovementsBackfillAndHistorySafety(t *testing.T) {
	pool := testdb.PostgresEmpty(t)
	ctx := context.Background()
	if err := database.MigrateFS(ctx, pool, upTo(t, "007_positions")); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, pool,
		`INSERT INTO users (id, email, password_hash) VALUES ('u1','me@example.com','x'), ('u2','other@example.com','x')`,
		`INSERT INTO workouts (id, name, user_id) VALUES ('wa','Push A','u1'), ('wb','Push B','u1'), ('wo','Theirs','u2')`,
		`INSERT INTO exercises (id, name, sets, reps, weight, workout_id) VALUES
			('ea','Bench Press',3,8,60,'wa'), ('eb','  bench press ',3,8,60,'wb'), ('ec','Squat',3,5,100,'wb'),
			('eo','Bench Press',3,8,60,'wo')`,
		`INSERT INTO workout_sessions (id, workout_id, user_id, is_active, ended_at) VALUES ('s1','wa','u1',false,NOW())`,
		`INSERT INTO session_exercises (id, session_id, exercise_id) VALUES ('se1','s1','ea')`,
		`INSERT INTO exercise_sets (id, session_exercise_id, reps, weight, completed) VALUES ('x1','se1',8,62.5,true)`,
	)

	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate 008: %v", err)
	}

	count := func(q string) int {
		var n int
		if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	if n := count(`SELECT COUNT(*) FROM movements WHERE user_id = 'u1'`); n != 2 {
		t.Errorf("u1 has %d movements, want 2 (bench merged, squat)", n)
	}
	if n := count(`SELECT COUNT(DISTINCT movement_id) FROM exercises WHERE id IN ('ea','eb')`); n != 1 {
		t.Error("Bench Press and '  bench press ' should share one movement")
	}
	if n := count(`SELECT COUNT(*) FROM exercises e JOIN movements m ON m.id = e.movement_id WHERE e.id = 'eo' AND m.user_id = 'u2'`); n != 1 {
		t.Error("another user's exercise must get that user's own movement")
	}
	var name, workoutName string
	pool.QueryRow(ctx, `SELECT se.name, ws.workout_name FROM session_exercises se JOIN workout_sessions ws ON ws.id = se.session_id WHERE se.id = 'se1'`).Scan(&name, &workoutName)
	if name != "Bench Press" || workoutName != "Push A" {
		t.Errorf("snapshot names = %q / %q, want Bench Press / Push A", name, workoutName)
	}

	testdb.Exec(t, pool, `DELETE FROM workouts WHERE id = 'wa'`)
	if n := count(`SELECT COUNT(*) FROM exercise_sets WHERE id = 'x1'`); n != 1 {
		t.Error("deleting the workout deleted its logged sets")
	}
	if n := count(`SELECT COUNT(*) FROM workout_sessions WHERE id = 's1' AND workout_id IS NULL`); n != 1 {
		t.Error("session should be kept with its workout link cleared")
	}
}
