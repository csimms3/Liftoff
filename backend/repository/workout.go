package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"liftoff/backend/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkoutRepository struct {
	db        *pgxpool.Pool
	sqlite    *sql.DB
	useSQLite bool
}

func NewWorkoutRepository(db *pgxpool.Pool, sqlite *sql.DB, useSQLite bool) *WorkoutRepository {
	return &WorkoutRepository{db: db, sqlite: sqlite, useSQLite: useSQLite}
}

// Workout operations
func (r *WorkoutRepository) CreateWorkout(ctx context.Context, name string) (*models.Workout, error) {
	id := uuid.New().String()
	now := time.Now()

	if r.useSQLite {
		return r.createWorkoutSQLite(ctx, id, name, now)
	}
	return r.createWorkoutPostgres(ctx, id, name, now)
}

func (r *WorkoutRepository) createWorkoutPostgres(ctx context.Context, id, name string, now time.Time) (*models.Workout, error) {
	query := `
		INSERT INTO workouts (id, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, created_at, updated_at
	`

	var workout models.Workout
	err := r.db.QueryRow(ctx, query, id, name, now, now).Scan(
		&workout.ID, &workout.Name, &workout.CreatedAt, &workout.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workout: %w", err)
	}

	return &workout, nil
}

func (r *WorkoutRepository) createWorkoutSQLite(ctx context.Context, id, name string, now time.Time) (*models.Workout, error) {
	query := `
		INSERT INTO workouts (id, name, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`

	_, err := r.sqlite.ExecContext(ctx, query, id, name, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create workout: %w", err)
	}

	return &models.Workout{
		ID:        id,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (r *WorkoutRepository) GetWorkouts(ctx context.Context) ([]*models.Workout, error) {
	if r.useSQLite {
		return r.getWorkoutsSQLite(ctx)
	}
	return r.getWorkoutsPostgres(ctx)
}

func (r *WorkoutRepository) getWorkoutsPostgres(ctx context.Context) ([]*models.Workout, error) {
	query := `
		SELECT id, name, created_at, updated_at
		FROM workouts
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get workouts: %w", err)
	}
	defer rows.Close()

	var workouts []*models.Workout
	for rows.Next() {
		var workout models.Workout
		err := rows.Scan(&workout.ID, &workout.Name, &workout.CreatedAt, &workout.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workout: %w", err)
		}
		workouts = append(workouts, &workout)
	}

	return workouts, nil
}

func (r *WorkoutRepository) getWorkoutsSQLite(ctx context.Context) ([]*models.Workout, error) {
	query := `
		SELECT id, name, created_at, updated_at
		FROM workouts
		ORDER BY created_at DESC
	`

	rows, err := r.sqlite.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get workouts: %w", err)
	}
	defer rows.Close()

	var workouts []*models.Workout
	for rows.Next() {
		var workout models.Workout
		err := rows.Scan(&workout.ID, &workout.Name, &workout.CreatedAt, &workout.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workout: %w", err)
		}
		workouts = append(workouts, &workout)
	}

	return workouts, nil
}

func (r *WorkoutRepository) GetWorkout(ctx context.Context, id string) (*models.Workout, error) {
	query := `
		SELECT id, name, created_at, updated_at
		FROM workouts
		WHERE id = $1
	`

	var workout models.Workout
	err := r.db.QueryRow(ctx, query, id).Scan(
		&workout.ID, &workout.Name, &workout.CreatedAt, &workout.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get workout: %w", err)
	}

	return &workout, nil
}

func (r *WorkoutRepository) UpdateWorkout(ctx context.Context, id, name string) (*models.Workout, error) {
	query := `
		UPDATE workouts
		SET name = $2, updated_at = $3
		WHERE id = $1
		RETURNING id, name, created_at, updated_at
	`

	var workout models.Workout
	err := r.db.QueryRow(ctx, query, id, name, time.Now()).Scan(
		&workout.ID, &workout.Name, &workout.CreatedAt, &workout.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update workout: %w", err)
	}

	return &workout, nil
}

func (r *WorkoutRepository) DeleteWorkout(ctx context.Context, id string) error {
	query := `DELETE FROM workouts WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete workout: %w", err)
	}
	return nil
}

// Exercise operations
func (r *WorkoutRepository) CreateExercise(ctx context.Context, exercise *models.Exercise) error {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO exercises (id, name, sets, reps, weight, workout_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(ctx, query, id, exercise.Name, exercise.Sets, exercise.Reps, exercise.Weight, exercise.WorkoutID, now, now)
	if err != nil {
		return fmt.Errorf("failed to create exercise: %w", err)
	}

	exercise.ID = id
	exercise.CreatedAt = now
	exercise.UpdatedAt = now
	return nil
}

func (r *WorkoutRepository) GetExercisesByWorkout(ctx context.Context, workoutID string) ([]*models.Exercise, error) {
	query := `
		SELECT id, name, sets, reps, weight, workout_id, created_at, updated_at
		FROM exercises
		WHERE workout_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, query, workoutID)
	if err != nil {
		return nil, fmt.Errorf("failed to get exercises: %w", err)
	}
	defer rows.Close()

	var exercises []*models.Exercise
	for rows.Next() {
		var exercise models.Exercise
		err := rows.Scan(
			&exercise.ID, &exercise.Name, &exercise.Sets, &exercise.Reps,
			&exercise.Weight, &exercise.WorkoutID, &exercise.CreatedAt, &exercise.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan exercise: %w", err)
		}
		exercises = append(exercises, &exercise)
	}

	return exercises, nil
}

func (r *WorkoutRepository) UpdateExercise(ctx context.Context, exercise *models.Exercise) error {
	query := `
		UPDATE exercises
		SET name = $2, sets = $3, reps = $4, weight = $5, updated_at = $6
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, exercise.ID, exercise.Name, exercise.Sets, exercise.Reps, exercise.Weight, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update exercise: %w", err)
	}

	return nil
}

func (r *WorkoutRepository) DeleteExercise(ctx context.Context, id string) error {
	query := `DELETE FROM exercises WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete exercise: %w", err)
	}
	return nil
}
