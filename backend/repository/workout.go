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
	if r.useSQLite {
		return r.deleteWorkoutSQLite(ctx, id)
	}
	return r.deleteWorkoutPostgres(ctx, id)
}

func (r *WorkoutRepository) deleteWorkoutPostgres(ctx context.Context, id string) error {
	query := `DELETE FROM workouts WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete workout: %w", err)
	}
	return nil
}

func (r *WorkoutRepository) deleteWorkoutSQLite(ctx context.Context, id string) error {
	query := `DELETE FROM workouts WHERE id = ?`
	_, err := r.sqlite.ExecContext(ctx, query, id)
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
	if r.useSQLite {
		return r.deleteExerciseSQLite(ctx, id)
	}
	return r.deleteExercisePostgres(ctx, id)
}

func (r *WorkoutRepository) deleteExercisePostgres(ctx context.Context, id string) error {
	query := `DELETE FROM exercises WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete exercise: %w", err)
	}
	return nil
}

func (r *WorkoutRepository) deleteExerciseSQLite(ctx context.Context, id string) error {
	query := `DELETE FROM exercises WHERE id = ?`
	_, err := r.sqlite.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete exercise: %w", err)
	}
	return nil
}

// GetWorkoutTemplates returns all available workout templates
func (r *WorkoutRepository) GetWorkoutTemplates(ctx context.Context) ([]*models.WorkoutTemplate, error) {
	if r.useSQLite {
		return r.getWorkoutTemplatesSQLite(ctx)
	}
	return r.getWorkoutTemplatesPostgres(ctx)
}

func (r *WorkoutRepository) getWorkoutTemplatesPostgres(ctx context.Context) ([]*models.WorkoutTemplate, error) {
	// For now, return predefined templates
	return r.getPredefinedTemplates(), nil
}

func (r *WorkoutRepository) getWorkoutTemplatesSQLite(ctx context.Context) ([]*models.WorkoutTemplate, error) {
	// For now, return predefined templates
	return r.getPredefinedTemplates(), nil
}

// getPredefinedTemplates returns a curated list of workout templates
func (r *WorkoutRepository) getPredefinedTemplates() []*models.WorkoutTemplate {
	return []*models.WorkoutTemplate{
		{
			ID:          "push-pull-legs",
			Name:        "Push Pull Legs",
			Type:        models.WorkoutTypeStrength,
			Description: "Classic 3-day split focusing on pushing, pulling, and leg movements",
			Difficulty:  "intermediate",
			Duration:    60,
			Exercises: []models.Exercise{
				{Name: "Bench Press", Sets: 4, Reps: 8, Weight: 0},
				{Name: "Overhead Press", Sets: 3, Reps: 10, Weight: 0},
				{Name: "Dips", Sets: 3, Reps: 12, Weight: 0},
				{Name: "Lateral Raises", Sets: 3, Reps: 15, Weight: 0},
			},
		},
		{
			ID:          "full-body-strength",
			Name:        "Full Body Strength",
			Type:        models.WorkoutTypeStrength,
			Description: "Complete full-body workout hitting all major muscle groups",
			Difficulty:  "beginner",
			Duration:    45,
			Exercises: []models.Exercise{
				{Name: "Squats", Sets: 3, Reps: 12, Weight: 0},
				{Name: "Push-ups", Sets: 3, Reps: 10, Weight: 0},
				{Name: "Rows", Sets: 3, Reps: 12, Weight: 0},
				{Name: "Plank", Sets: 3, Reps: 1, Weight: 0},
			},
		},
		{
			ID:          "hiit-cardio",
			Name:        "HIIT Cardio",
			Type:        models.WorkoutTypeHIIT,
			Description: "High-intensity interval training for cardiovascular fitness",
			Difficulty:  "advanced",
			Duration:    30,
			Exercises: []models.Exercise{
				{Name: "Burpees", Sets: 4, Reps: 20, Weight: 0},
				{Name: "Mountain Climbers", Sets: 4, Reps: 30, Weight: 0},
				{Name: "Jump Squats", Sets: 4, Reps: 15, Weight: 0},
				{Name: "High Knees", Sets: 4, Reps: 30, Weight: 0},
			},
		},
		{
			ID:          "upper-body-focus",
			Name:        "Upper Body Focus",
			Type:        models.WorkoutTypeStrength,
			Description: "Targeted upper body workout for chest, back, and arms",
			Difficulty:  "intermediate",
			Duration:    50,
			Exercises: []models.Exercise{
				{Name: "Pull-ups", Sets: 4, Reps: 8, Weight: 0},
				{Name: "Dumbbell Rows", Sets: 3, Reps: 12, Weight: 0},
				{Name: "Diamond Push-ups", Sets: 3, Reps: 12, Weight: 0},
				{Name: "Bicep Curls", Sets: 3, Reps: 15, Weight: 0},
			},
		},
		{
			ID:          "core-strength",
			Name:        "Core Strength",
			Type:        models.WorkoutTypeStrength,
			Description: "Comprehensive core workout for stability and strength",
			Difficulty:  "beginner",
			Duration:    25,
			Exercises: []models.Exercise{
				{Name: "Crunches", Sets: 3, Reps: 20, Weight: 0},
				{Name: "Russian Twists", Sets: 3, Reps: 20, Weight: 0},
				{Name: "Leg Raises", Sets: 3, Reps: 15, Weight: 0},
				{Name: "Side Plank", Sets: 3, Reps: 1, Weight: 0},
			},
		},
		{
			ID:          "endurance-run",
			Name:        "Endurance Run",
			Type:        models.WorkoutTypeEndurance,
			Description: "Steady-state cardio for building endurance",
			Difficulty:  "beginner",
			Duration:    45,
			Exercises: []models.Exercise{
				{Name: "Running", Sets: 1, Reps: 1, Weight: 0},
				{Name: "Walking", Sets: 1, Reps: 1, Weight: 0},
			},
		},
	}
}

// CreateWorkoutFromTemplate creates a new workout based on a template
func (r *WorkoutRepository) CreateWorkoutFromTemplate(ctx context.Context, templateID string, name string) (*models.Workout, error) {
	templates := r.getPredefinedTemplates()
	var template *models.WorkoutTemplate
	
	for _, t := range templates {
		if t.ID == templateID {
			template = t
			break
		}
	}
	
	if template == nil {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}
	
	// Create the workout
	workout, err := r.CreateWorkout(ctx, name)
	if err != nil {
		return nil, err
	}
	
	// Add exercises from template
	for _, exercise := range template.Exercises {
		exercise.WorkoutID = workout.ID
		err = r.CreateExercise(ctx, &exercise)
		if err != nil {
			return nil, fmt.Errorf("failed to create exercise %s: %w", exercise.Name, err)
		}
	}
	
	return workout, nil
}
