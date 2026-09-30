package repository_test

import (
	"context"
	"strconv"
	"testing"

	"liftoff/backend/internal/testdb"
	"liftoff/backend/models"
	"liftoff/backend/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

// env is the repositories on a fresh, migrated Postgres schema, plus raw SQL access
// for setup and checks.
type env struct {
	pool     *pgxpool.Pool
	users    *repository.UserRepository
	workouts *repository.WorkoutRepository
	sessions *repository.SessionRepository
	routines *repository.RoutineRepository
}

// withDB runs fn against a fresh, migrated Postgres schema.
func withDB(t *testing.T, fn func(t *testing.T, e *env)) {
	pool := testdb.Postgres(t)
	workouts := repository.NewWorkoutRepository(pool)
	fn(t, &env{
		pool:     pool,
		users:    repository.NewUserRepository(pool),
		workouts: workouts,
		sessions: repository.NewSessionRepository(pool),
		routines: repository.NewRoutineRepository(pool, workouts),
	})
}

// exec runs raw SQL on the test schema.
func (e *env) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// count runs a COUNT query.
func (e *env) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
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
	w, err := e.workouts.CreateWorkout(ctx, userID, name, "")
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
