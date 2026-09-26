package repository_test

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"testing"

	"liftoff/backend/internal/testdb"
	"liftoff/backend/models"
	"liftoff/backend/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

// env is one backend's repositories plus raw SQL access for setup and checks.
type env struct {
	name     string
	pool     *pgxpool.Pool
	sqlite   *sql.DB
	isSQLite bool
	users    *repository.UserRepository
	workouts *repository.WorkoutRepository
	sessions *repository.SessionRepository
	routines *repository.RoutineRepository
}

// forEachBackend runs fn against a fresh SQLite database and, when
// LIFTOFF_TEST_DATABASE_URL is set, a fresh Postgres schema.
func forEachBackend(t *testing.T, fn func(t *testing.T, e *env)) {
	t.Run("sqlite", func(t *testing.T) {
		db := testdb.SQLite(t)
		fn(t, newEnv("sqlite", nil, db.GetSQLite(), true))
	})
	t.Run("postgres", func(t *testing.T) {
		fn(t, newEnv("postgres", testdb.Postgres(t), nil, false))
	})
}

func newEnv(name string, pool *pgxpool.Pool, sqlite *sql.DB, isSQLite bool) *env {
	workouts := repository.NewWorkoutRepository(pool, sqlite, isSQLite)
	return &env{
		name: name, pool: pool, sqlite: sqlite, isSQLite: isSQLite,
		users:    repository.NewUserRepository(pool, sqlite, isSQLite),
		workouts: workouts,
		sessions: repository.NewSessionRepository(pool, sqlite, isSQLite),
		routines: repository.NewRoutineRepository(pool, sqlite, isSQLite, workouts),
	}
}

// exec runs raw SQL written with ? placeholders on either backend.
func (e *env) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	var err error
	if e.isSQLite {
		_, err = e.sqlite.Exec(query, args...)
	} else {
		_, err = e.pool.Exec(context.Background(), pgRebind(query), args...)
	}
	if err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// count runs a COUNT query written with ? placeholders.
func (e *env) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	var err error
	if e.isSQLite {
		err = e.sqlite.QueryRow(query, args...).Scan(&n)
	} else {
		err = e.pool.QueryRow(context.Background(), pgRebind(query), args...).Scan(&n)
	}
	if err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func pgRebind(q string) string {
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$" + strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (e *env) user(t *testing.T, email string) string {
	t.Helper()
	u, err := e.users.CreateUser(context.Background(), email, "hash")
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

// workout creates a workout for userID with one exercise per sets value.
func (e *env) workout(t *testing.T, userID, name string, sets ...int) *models.Workout {
	t.Helper()
	ctx := context.Background()
	w, err := e.workouts.CreateWorkout(ctx, userID, name)
	if err != nil {
		t.Fatal(err)
	}
	for i, n := range sets {
		ex := &models.Exercise{Name: name + "-ex" + strconv.Itoa(i), Sets: n, Reps: 8, Weight: 50, WorkoutID: w.ID}
		if err := e.workouts.CreateExercise(ctx, userID, ex); err != nil {
			t.Fatal(err)
		}
	}
	w, err = e.workouts.GetWorkout(ctx, userID, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	return w
}
