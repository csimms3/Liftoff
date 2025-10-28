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

/**
 * WorkoutRepository Package
 *
 * Handles all database operations related to workouts, exercises, and templates.
 * Supports both PostgreSQL and SQLite databases with automatic routing based on
 * the active database connection.
 *
 * Features:
 * - CRUD operations for workouts and exercises
 * - Workout template management
 * - Exercise template library
 * - Database-agnostic operations
 * - Proper error handling and logging
 */

// WorkoutRepository manages workout-related database operations
type WorkoutRepository struct {
	db        *pgxpool.Pool // PostgreSQL connection pool
	sqlite    *sql.DB       // SQLite database connection
	useSQLite bool          // Flag indicating which database to use
}

/**
 * NewWorkoutRepository creates a new workout repository instance
 *
 * Args:
 * - db: PostgreSQL connection pool
 * - sqlite: SQLite database connection
 * - useSQLite: Boolean flag indicating which database to use
 *
 * Returns:
 * - *WorkoutRepository: Configured repository instance
 */
func NewWorkoutRepository(db *pgxpool.Pool, sqlite *sql.DB, useSQLite bool) *WorkoutRepository {
	if useSQLite {
		return &WorkoutRepository{db: nil, sqlite: sqlite, useSQLite: true}
	}
	return &WorkoutRepository{db: db, sqlite: nil, useSQLite: false}
}

/**
 * CreateWorkout creates a new workout in the database
 *
 * Generates a unique UUID and timestamp, then delegates to the appropriate
 * database implementation based on the useSQLite flag.
 *
 * Args:
 * - ctx: Context for the operation
 * - name: Name of the workout to create
 *
 * Returns:
 * - *models.Workout: Created workout with generated ID and timestamps
 * - error: Creation error if any
 */
func (r *WorkoutRepository) CreateWorkout(ctx context.Context, name string) (*models.Workout, error) {
	id := uuid.New().String()
	now := time.Now()

	if r.useSQLite {
		return r.createWorkoutSQLite(ctx, id, name, now)
	}
	return r.createWorkoutPostgres(ctx, id, name, now)
}

/**
 * createWorkoutPostgres creates a workout in PostgreSQL database
 *
 * Uses parameterized queries with proper error handling and returns
 * the created workout with all fields populated.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: Generated UUID for the workout
 * - name: Name of the workout
 * - now: Current timestamp
 *
 * Returns:
 * - *models.Workout: Created workout with all fields
 * - error: Database error if any
 */
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

/**
 * createWorkoutSQLite creates a workout in SQLite database
 *
 * Uses SQLite-specific parameter syntax (?) and manually constructs
 * the workout object since SQLite doesn't support RETURNING clause.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: Generated UUID for the workout
 * - name: Name of the workout
 * - now: Current timestamp
 *
 * Returns:
 * - *models.Workout: Created workout with all fields
 * - error: Database error if any
 */
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

/**
 * GetWorkouts retrieves all workouts from the database
 *
 * Delegates to the appropriate database implementation and returns
 * workouts ordered by creation date (newest first).
 *
 * Args:
 * - ctx: Context for the operation
 *
 * Returns:
 * - []*models.Workout: List of all workouts
 * - error: Database error if any
 */
func (r *WorkoutRepository) GetWorkouts(ctx context.Context) ([]*models.Workout, error) {
	if r.useSQLite {
		return r.getWorkoutsSQLite(ctx)
	}
	return r.getWorkoutsPostgres(ctx)
}

/**
 * getWorkoutsPostgres retrieves workouts from PostgreSQL database
 *
 * Uses parameterized queries and proper row scanning with error handling.
 * Returns workouts ordered by creation date descending.
 *
 * Args:
 * - ctx: Context for the operation
 *
 * Returns:
 * - []*models.Workout: List of workouts from PostgreSQL
 * - error: Database error if any
 */
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

/**
 * getWorkoutsSQLite retrieves workouts from SQLite database
 *
 * Uses SQLite-specific parameter syntax (?) and proper row scanning with error handling.
 * Returns workouts ordered by creation date descending.
 *
 * Args:
 * - ctx: Context for the operation
 *
 * Returns:
 * - []*models.Workout: List of workouts from SQLite
 * - error: Database error if any
 */
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

/**
 * GetWorkout retrieves a single workout by its ID from the database
 *
 * Delegates to the appropriate database implementation based on the useSQLite flag.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the workout to retrieve
 *
 * Returns:
 * - *models.Workout: Retrieved workout
 * - error: Database error if any
 */
func (r *WorkoutRepository) GetWorkout(ctx context.Context, id string) (*models.Workout, error) {
	var workout *models.Workout
	var err error
	
	if r.useSQLite {
		workout, err = r.getWorkoutSQLite(ctx, id)
	} else {
		workout, err = r.getWorkoutPostgres(ctx, id)
	}
	
	if err != nil {
		return nil, err
	}
	
	// Load exercises for this workout
	exercisePtrs, err := r.GetExercisesByWorkout(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load exercises: %w", err)
	}
	
	// Convert []*Exercise to []Exercise
	exercises := make([]models.Exercise, len(exercisePtrs))
	for i, exercisePtr := range exercisePtrs {
		exercises[i] = *exercisePtr
	}
	
	workout.Exercises = exercises
	return workout, nil
}

/**
 * getWorkoutPostgres retrieves a workout from PostgreSQL database
 *
 * Uses parameterized query with error handling.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the workout to retrieve
 *
 * Returns:
 * - *models.Workout: Retrieved workout
 * - error: Database error if any
 */
func (r *WorkoutRepository) getWorkoutPostgres(ctx context.Context, id string) (*models.Workout, error) {
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

/**
 * getWorkoutSQLite retrieves a workout from SQLite database
 *
 * Uses SQLite-specific parameter syntax (?) and error handling.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the workout to retrieve
 *
 * Returns:
 * - *models.Workout: Retrieved workout
 * - error: Database error if any
 */
func (r *WorkoutRepository) getWorkoutSQLite(ctx context.Context, id string) (*models.Workout, error) {
	query := `
		SELECT id, name, created_at, updated_at
		FROM workouts
		WHERE id = ?
	`

	var workout models.Workout
	err := r.sqlite.QueryRowContext(ctx, query, id).Scan(
		&workout.ID, &workout.Name, &workout.CreatedAt, &workout.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get workout: %w", err)
	}

	return &workout, nil
}

/**
 * UpdateWorkout updates an existing workout in the database
 *
 * Uses parameterized query with error handling and returns the updated workout.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the workout to update
 * - name: New name for the workout
 *
 * Returns:
 * - *models.Workout: Updated workout
 * - error: Database error if any
 */
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

/**
 * DeleteWorkout removes a workout from the database
 *
 * Delegates to the appropriate database implementation based on the useSQLite flag.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the workout to delete
 *
 * Returns:
 * - error: Database error if any
 */
func (r *WorkoutRepository) DeleteWorkout(ctx context.Context, id string) error {
	if r.useSQLite {
		return r.deleteWorkoutSQLite(ctx, id)
	}
	return r.deleteWorkoutPostgres(ctx, id)
}

/**
 * deleteWorkoutPostgres deletes a workout from PostgreSQL database
 *
 * Uses parameterized query with error handling.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the workout to delete
 *
 * Returns:
 * - error: Database error if any
 */
func (r *WorkoutRepository) deleteWorkoutPostgres(ctx context.Context, id string) error {
	query := `DELETE FROM workouts WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete workout: %w", err)
	}
	return nil
}

/**
 * deleteWorkoutSQLite deletes a workout from SQLite database
 *
 * Uses SQLite-specific parameter syntax (?) and error handling.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the workout to delete
 *
 * Returns:
 * - error: Database error if any
 */
func (r *WorkoutRepository) deleteWorkoutSQLite(ctx context.Context, id string) error {
	query := `DELETE FROM workouts WHERE id = ?`
	_, err := r.sqlite.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete workout: %w", err)
	}
	return nil
}

/**
 * Exercise operations
 *
 * Handles CRUD operations for exercises within workouts.
 */

/**
 * CreateExercise creates a new exercise in the database
 *
 * Generates a unique UUID and timestamp, then delegates to the appropriate
 * database implementation based on the useSQLite flag.
 *
 * Args:
 * - ctx: Context for the operation
 * - exercise: Pointer to the exercise model to create
 *
 * Returns:
 * - error: Creation error if any
 */
func (r *WorkoutRepository) CreateExercise(ctx context.Context, exercise *models.Exercise) error {
	id := uuid.New().String()
	now := time.Now()

	if r.useSQLite {
		return r.createExerciseSQLite(ctx, id, exercise, now)
	}
	return r.createExercisePostgres(ctx, id, exercise, now)
}

/**
 * createExercisePostgres creates an exercise in PostgreSQL database
 *
 * Uses parameterized queries with proper error handling.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: Generated UUID for the exercise
 * - exercise: Pointer to the exercise model
 * - now: Current timestamp
 *
 * Returns:
 * - error: Database error if any
 */
func (r *WorkoutRepository) createExercisePostgres(ctx context.Context, id string, exercise *models.Exercise, now time.Time) error {
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

/**
 * createExerciseSQLite creates an exercise in SQLite database
 *
 * Uses SQLite-specific parameter syntax (?) and error handling.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: Generated UUID for the exercise
 * - exercise: Pointer to the exercise model
 * - now: Current timestamp
 *
 * Returns:
 * - error: Database error if any
 */
func (r *WorkoutRepository) createExerciseSQLite(ctx context.Context, id string, exercise *models.Exercise, now time.Time) error {
	query := `
		INSERT INTO exercises (id, name, sets, reps, weight, workout_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.sqlite.ExecContext(ctx, query, id, exercise.Name, exercise.Sets, exercise.Reps, exercise.Weight, exercise.WorkoutID, now, now)
	if err != nil {
		return fmt.Errorf("failed to create exercise: %w", err)
	}

	exercise.ID = id
	exercise.CreatedAt = now
	exercise.UpdatedAt = now
	return nil
}

/**
 * GetExercisesByWorkout retrieves all exercises for a specific workout from the database
 *
 * Delegates to the appropriate database implementation based on the useSQLite flag.
 *
 * Args:
 * - ctx: Context for the operation
 * - workoutID: ID of the workout to retrieve exercises for
 *
 * Returns:
 * - []*models.Exercise: List of exercises for the workout
 * - error: Database error if any
 */
func (r *WorkoutRepository) GetExercisesByWorkout(ctx context.Context, workoutID string) ([]*models.Exercise, error) {
	if r.useSQLite {
		return r.getExercisesByWorkoutSQLite(ctx, workoutID)
	}
	return r.getExercisesByWorkoutPostgres(ctx, workoutID)
}

/**
 * getExercisesByWorkoutPostgres retrieves exercises from PostgreSQL database
 *
 * Uses parameterized query with error handling.
 *
 * Args:
 * - ctx: Context for the operation
 * - workoutID: ID of the workout to retrieve exercises for
 *
 * Returns:
 * - []*models.Exercise: List of exercises for the workout
 * - error: Database error if any
 */
func (r *WorkoutRepository) getExercisesByWorkoutPostgres(ctx context.Context, workoutID string) ([]*models.Exercise, error) {
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

/**
 * getExercisesByWorkoutSQLite retrieves exercises from SQLite database
 *
 * Uses SQLite-specific parameter syntax (?) and error handling.
 *
 * Args:
 * - ctx: Context for the operation
 * - workoutID: ID of the workout to retrieve exercises for
 *
 * Returns:
 * - []*models.Exercise: List of exercises for the workout
 * - error: Database error if any
 */
func (r *WorkoutRepository) getExercisesByWorkoutSQLite(ctx context.Context, workoutID string) ([]*models.Exercise, error) {
	query := `
		SELECT id, name, sets, reps, weight, workout_id, created_at, updated_at
		FROM exercises
		WHERE workout_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.sqlite.QueryContext(ctx, query, workoutID)
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

// GetExercise retrieves a single exercise by ID
func (r *WorkoutRepository) GetExercise(ctx context.Context, exerciseID string) (*models.Exercise, error) {
	if r.useSQLite {
		return r.getExerciseSQLite(ctx, exerciseID)
	}
	return r.getExercisePostgres(ctx, exerciseID)
}

func (r *WorkoutRepository) getExercisePostgres(ctx context.Context, exerciseID string) (*models.Exercise, error) {
	query := `
		SELECT id, name, sets, reps, weight, workout_id, created_at, updated_at
		FROM exercises
		WHERE id = $1
	`

	var exercise models.Exercise
	err := r.db.QueryRow(ctx, query, exerciseID).Scan(
		&exercise.ID, &exercise.Name, &exercise.Sets, &exercise.Reps,
		&exercise.Weight, &exercise.WorkoutID, &exercise.CreatedAt, &exercise.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get exercise: %w", err)
	}

	return &exercise, nil
}

func (r *WorkoutRepository) getExerciseSQLite(ctx context.Context, exerciseID string) (*models.Exercise, error) {
	query := `
		SELECT id, name, sets, reps, weight, workout_id, created_at, updated_at
		FROM exercises
		WHERE id = ?
	`

	var exercise models.Exercise
	err := r.sqlite.QueryRowContext(ctx, query, exerciseID).Scan(
		&exercise.ID, &exercise.Name, &exercise.Sets, &exercise.Reps,
		&exercise.Weight, &exercise.WorkoutID, &exercise.CreatedAt, &exercise.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get exercise: %w", err)
	}

	return &exercise, nil
}

/**
 * UpdateExercise updates an existing exercise in the database
 *
 * Uses parameterized query with error handling.
 *
 * Args:
 * - ctx: Context for the operation
 * - exercise: Pointer to the exercise model to update
 *
 * Returns:
 * - error: Database error if any
 */
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

/**
 * DeleteExercise removes an exercise from the database
 *
 * Delegates to the appropriate database implementation based on the useSQLite flag.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the exercise to delete
 *
 * Returns:
 * - error: Database error if any
 */
func (r *WorkoutRepository) DeleteExercise(ctx context.Context, id string) error {
	if r.useSQLite {
		return r.deleteExerciseSQLite(ctx, id)
	}
	return r.deleteExercisePostgres(ctx, id)
}

/**
 * deleteExercisePostgres deletes an exercise from PostgreSQL database
 *
 * Uses parameterized query with error handling.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the exercise to delete
 *
 * Returns:
 * - error: Database error if any
 */
func (r *WorkoutRepository) deleteExercisePostgres(ctx context.Context, id string) error {
	query := `DELETE FROM exercises WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete exercise: %w", err)
	}
	return nil
}

/**
 * deleteExerciseSQLite deletes an exercise from SQLite database
 *
 * Uses SQLite-specific parameter syntax (?) and error handling.
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the exercise to delete
 *
 * Returns:
 * - error: Database error if any
 */
func (r *WorkoutRepository) deleteExerciseSQLite(ctx context.Context, id string) error {
	query := `DELETE FROM exercises WHERE id = ?`
	_, err := r.sqlite.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete exercise: %w", err)
	}
	return nil
}

/**
 * GetWorkoutTemplates returns all available workout templates
 *
 * Retrieves templates from the appropriate database implementation.
 *
 * Args:
 * - ctx: Context for the operation
 *
 * Returns:
 * - []*models.WorkoutTemplate: List of workout templates
 * - error: Database error if any
 */
func (r *WorkoutRepository) GetWorkoutTemplates(ctx context.Context) ([]*models.WorkoutTemplate, error) {
	if r.useSQLite {
		return r.getWorkoutTemplatesSQLite(ctx)
	}
	return r.getWorkoutTemplatesPostgres(ctx)
}

/**
 * getWorkoutTemplatesPostgres retrieves workout templates from PostgreSQL database
 *
 * For now, returns predefined templates.
 *
 * Args:
 * - ctx: Context for the operation
 *
 * Returns:
 * - []*models.WorkoutTemplate: List of workout templates from PostgreSQL
 * - error: Database error if any
 */
func (r *WorkoutRepository) getWorkoutTemplatesPostgres(ctx context.Context) ([]*models.WorkoutTemplate, error) {
	// For now, return predefined templates
	return r.getPredefinedTemplates(), nil
}

/**
 * getWorkoutTemplatesSQLite retrieves workout templates from SQLite database
 *
 * For now, returns predefined templates.
 *
 * Args:
 * - ctx: Context for the operation
 *
 * Returns:
 * - []*models.WorkoutTemplate: List of workout templates from SQLite
 * - error: Database error if any
 */
func (r *WorkoutRepository) getWorkoutTemplatesSQLite(ctx context.Context) ([]*models.WorkoutTemplate, error) {
	// For now, return predefined templates
	return r.getPredefinedTemplates(), nil
}

/**
 * GetExerciseTemplates returns all available exercise templates
 *
 * Returns a predefined list of exercise templates.
 *
 * Args:
 * - ctx: Context for the operation
 *
 * Returns:
 * - []*models.ExerciseTemplate: List of exercise templates
 * - error: Database error if any
 */
func (r *WorkoutRepository) GetExerciseTemplates(ctx context.Context) ([]*models.ExerciseTemplate, error) {
	return r.getPredefinedExerciseTemplates(), nil
}

/**
 * getPredefinedExerciseTemplates returns a curated list of exercise templates
 *
 * Returns a predefined list of exercise templates.
 *
 * Returns:
 * - []*models.ExerciseTemplate: List of exercise templates
 */
func (r *WorkoutRepository) getPredefinedExerciseTemplates() []*models.ExerciseTemplate {
	return []*models.ExerciseTemplate{
		// Chest exercises
		{Name: "Barbell Bench Press", Category: "Chest", DefaultSets: 4, DefaultReps: 8, DefaultWeight: 0},
		{Name: "Dumbbell Bench Press", Category: "Chest", DefaultSets: 3, DefaultReps: 10, DefaultWeight: 0},
		{Name: "Push-ups", Category: "Chest", DefaultSets: 3, DefaultReps: 15, DefaultWeight: 0},
		{Name: "Incline Dumbbell Press", Category: "Chest", DefaultSets: 3, DefaultReps: 10, DefaultWeight: 0},

		// Back exercises
		{Name: "Pull-ups", Category: "Back", DefaultSets: 4, DefaultReps: 8, DefaultWeight: 0},
		{Name: "Barbell Rows", Category: "Back", DefaultSets: 4, DefaultReps: 10, DefaultWeight: 0},
		{Name: "Dumbbell Rows", Category: "Back", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 0},
		{Name: "Lat Pulldowns", Category: "Back", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 0},

		// Shoulder exercises
		{Name: "Overhead Press", Category: "Shoulders", DefaultSets: 3, DefaultReps: 10, DefaultWeight: 0},
		{Name: "Lateral Raises", Category: "Shoulders", DefaultSets: 3, DefaultReps: 15, DefaultWeight: 0},
		{Name: "Front Raises", Category: "Shoulders", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 0},
		{Name: "Dumbbell Shoulder Press", Category: "Shoulders", DefaultSets: 3, DefaultReps: 10, DefaultWeight: 0},

		// Arm exercises
		{Name: "Bicep Curls", Category: "Arms", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 0},
		{Name: "Tricep Dips", Category: "Arms", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 0},
		{Name: "Hammer Curls", Category: "Arms", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 0},
		{Name: "Tricep Pushdowns", Category: "Arms", DefaultSets: 3, DefaultReps: 15, DefaultWeight: 0},

		// Leg exercises
		{Name: "Barbell Squats", Category: "Legs", DefaultSets: 4, DefaultReps: 8, DefaultWeight: 0},
		{Name: "Deadlifts", Category: "Legs", DefaultSets: 4, DefaultReps: 6, DefaultWeight: 0},
		{Name: "Lunges", Category: "Legs", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 0},
		{Name: "Leg Press", Category: "Legs", DefaultSets: 3, DefaultReps: 10, DefaultWeight: 0},

		// Core exercises
		{Name: "Plank", Category: "Core", DefaultSets: 3, DefaultReps: 1, DefaultWeight: 0},
		{Name: "Crunches", Category: "Core", DefaultSets: 3, DefaultReps: 20, DefaultWeight: 0},
		{Name: "Russian Twists", Category: "Core", DefaultSets: 3, DefaultReps: 20, DefaultWeight: 0},
		{Name: "Leg Raises", Category: "Core", DefaultSets: 3, DefaultReps: 15, DefaultWeight: 0},

		// Cardio exercises
		{Name: "Running", Category: "Cardio", DefaultSets: 1, DefaultReps: 1, DefaultWeight: 0},
		{Name: "Cycling", Category: "Cardio", DefaultSets: 1, DefaultReps: 1, DefaultWeight: 0},
		{Name: "Jump Rope", Category: "Cardio", DefaultSets: 3, DefaultReps: 1, DefaultWeight: 0},
		{Name: "Burpees", Category: "Cardio", DefaultSets: 4, DefaultReps: 15, DefaultWeight: 0},
	}
}

/**
 * getPredefinedTemplates returns a curated list of workout templates
 *
 * Returns a predefined list of workout templates.
 *
 * Returns:
 * - []*models.WorkoutTemplate: List of workout templates
 */
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

/**
 * CreateWorkoutFromTemplate creates a new workout based on a template
 *
 * Retrieves a template by its ID, creates a new workout, and adds exercises
 * from the template to the new workout.
 *
 * Args:
 * - ctx: Context for the operation
 * - templateID: ID of the template to use
 * - name: Name for the new workout
 *
 * Returns:
 * - *models.Workout: Created workout with exercises from template
 * - error: Creation error if any
 */
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

/**
 * CreateDinoGameScore creates a new dino game score in the database
 */
func (r *WorkoutRepository) CreateDinoGameScore(ctx context.Context, score int) (*models.DinoGameScore, error) {
	id := uuid.New().String()
	now := time.Now()

	if r.useSQLite {
		return r.createDinoGameScoreSQLite(ctx, id, score, now)
	}
	return r.createDinoGameScorePostgres(ctx, id, score, now)
}

func (r *WorkoutRepository) createDinoGameScorePostgres(ctx context.Context, id string, score int, now time.Time) (*models.DinoGameScore, error) {
	query := `
		INSERT INTO dino_game_scores (id, score, created_at)
		VALUES ($1, $2, $3)
		RETURNING id, score, created_at
	`

	var dinoScore models.DinoGameScore
	err := r.db.QueryRow(ctx, query, id, score, now).Scan(
		&dinoScore.ID, &dinoScore.Score, &dinoScore.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create dino game score: %w", err)
	}

	return &dinoScore, nil
}

func (r *WorkoutRepository) createDinoGameScoreSQLite(ctx context.Context, id string, score int, now time.Time) (*models.DinoGameScore, error) {
	query := `
		INSERT INTO dino_game_scores (id, score, created_at)
		VALUES (?, ?, ?)
	`

	_, err := r.sqlite.ExecContext(ctx, query, id, score, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create dino game score: %w", err)
	}

	return &models.DinoGameScore{
		ID:        id,
		Score:     score,
		CreatedAt: now,
	}, nil
}

/**
 * GetDinoGameHighScore retrieves the highest score from the dino game
 */
func (r *WorkoutRepository) GetDinoGameHighScore(ctx context.Context) (int, error) {
	if r.useSQLite {
		return r.getDinoGameHighScoreSQLite(ctx)
	}
	return r.getDinoGameHighScorePostgres(ctx)
}

func (r *WorkoutRepository) getDinoGameHighScorePostgres(ctx context.Context) (int, error) {
	query := `
		SELECT COALESCE(MAX(score), 0)
		FROM dino_game_scores
	`

	var highScore int
	err := r.db.QueryRow(ctx, query).Scan(&highScore)
	if err != nil {
		return 0, fmt.Errorf("failed to get high score: %w", err)
	}

	return highScore, nil
}

func (r *WorkoutRepository) getDinoGameHighScoreSQLite(ctx context.Context) (int, error) {
	query := `
		SELECT COALESCE(MAX(score), 0)
		FROM dino_game_scores
	`

	var highScore int
	err := r.sqlite.QueryRowContext(ctx, query).Scan(&highScore)
	if err != nil {
		return 0, fmt.Errorf("failed to get high score: %w", err)
	}

	return highScore, nil
}
