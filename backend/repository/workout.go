package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"liftoff/backend/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

/**
 * WorkoutRepository Package
 *
 * Handles all database operations related to workouts, exercises, and templates.
 *
 * Features:
 * - CRUD operations for workouts and exercises
 * - Workout template management
 * - Exercise template library
 * - Proper error handling and logging
 */

// WorkoutRepository manages workout-related database operations
type WorkoutRepository struct {
	db *pgxpool.Pool // PostgreSQL connection pool
}

/**
 * NewWorkoutRepository creates a new workout repository instance
 *
 * Args:
 * - db: PostgreSQL connection pool
 *
 * Returns:
 * - *WorkoutRepository: Configured repository instance
 */
func NewWorkoutRepository(db *pgxpool.Pool) *WorkoutRepository {
	return &WorkoutRepository{db: db}
}

/**
 * CreateWorkout creates a new workout in the database
 *
 * Generates a unique UUID and timestamp.
 *
 * Args:
 * - ctx: Context for the operation
 * - name: Name of the workout to create
 *
 * Returns:
 * - *models.Workout: Created workout with generated ID and timestamps
 * - error: Creation error if any
 */
func (r *WorkoutRepository) CreateWorkout(ctx context.Context, userID, name string) (*models.Workout, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO workouts (id, user_id, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, name, created_at, updated_at
	`

	var workout models.Workout
	err := r.db.QueryRow(ctx, query, id, userID, name, now, now).Scan(
		&workout.ID, &workout.UserID, &workout.Name, &workout.CreatedAt, &workout.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workout: %w", err)
	}

	return &workout, nil
}

/**
 * GetWorkouts retrieves all workouts from the database
 *
 * Returns workouts ordered by creation date (newest first).
 *
 * Args:
 * - ctx: Context for the operation
 *
 * Returns:
 * - []*models.Workout: List of all workouts
 * - error: Database error if any
 */
func (r *WorkoutRepository) GetWorkouts(ctx context.Context, userID string) ([]*models.Workout, error) {
	query := `
		SELECT id, user_id, name, created_at, updated_at
		FROM workouts
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workouts: %w", err)
	}
	defer rows.Close()

	var workouts []*models.Workout
	for rows.Next() {
		var workout models.Workout
		err := rows.Scan(&workout.ID, &workout.UserID, &workout.Name, &workout.CreatedAt, &workout.UpdatedAt)
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
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the workout to retrieve
 *
 * Returns:
 * - *models.Workout: Retrieved workout
 * - error: Database error if any
 */
func (r *WorkoutRepository) GetWorkout(ctx context.Context, userID, id string) (*models.Workout, error) {
	workout, err := r.getWorkout(ctx, userID, id)
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
 * getWorkout retrieves a workout row (without exercises)
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
func (r *WorkoutRepository) getWorkout(ctx context.Context, userID, id string) (*models.Workout, error) {
	query := `
		SELECT id, user_id, name, created_at, updated_at
		FROM workouts
		WHERE id = $1 AND user_id = $2
	`

	var workout models.Workout
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&workout.ID, &workout.UserID, &workout.Name, &workout.CreatedAt, &workout.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get workout: %w", err)
	}

	return &workout, nil
}

/**
 * DeleteWorkout removes a workout from the database
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the workout to delete
 *
 * Returns:
 * - error: Database error if any
 */
func (r *WorkoutRepository) DeleteWorkout(ctx context.Context, userID, id string) error {
	query := `DELETE FROM workouts WHERE id = $1 AND user_id = $2`
	_, err := r.db.Exec(ctx, query, id, userID)
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
 * Generates a unique UUID and timestamp.
 *
 * Args:
 * - ctx: Context for the operation
 * - exercise: Pointer to the exercise model to create
 *
 * Returns:
 * - error: Creation error if any
 */
func (r *WorkoutRepository) CreateExercise(ctx context.Context, userID string, exercise *models.Exercise) error {
	// Verify workout belongs to user
	_, err := r.GetWorkout(ctx, userID, exercise.WorkoutID)
	if err != nil {
		return fmt.Errorf("workout not found or access denied: %w", err)
	}

	// The movement and the exercise are created together or not at all.
	exercise.Name = NormalizeExerciseName(exercise.Name)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	movementID, err := findOrCreateMovement(ctx, tx, userID, exercise.Name)
	if err != nil {
		return err
	}
	id := uuid.New().String()
	now := time.Now()
	if _, err := tx.Exec(ctx, `
		INSERT INTO exercises (id, name, sets, reps, weight, workout_id, movement_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id, exercise.Name, exercise.Sets, exercise.Reps, exercise.Weight, exercise.WorkoutID, movementID, now, now); err != nil {
		return fmt.Errorf("failed to create exercise: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to create exercise: %w", err)
	}

	exercise.ID = id
	exercise.MovementID = movementID
	exercise.CreatedAt = now
	exercise.UpdatedAt = now
	return nil
}

/**
 * GetExercisesByWorkout retrieves all exercises for a specific workout from the database
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
	query := `
		SELECT id, name, sets, reps, weight, workout_id, movement_id, created_at, updated_at
		FROM exercises
		WHERE workout_id = $1
		ORDER BY created_at, id
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
			&exercise.Weight, &exercise.WorkoutID, &exercise.MovementID, &exercise.CreatedAt, &exercise.UpdatedAt,
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
	query := `
		SELECT id, name, sets, reps, weight, workout_id, movement_id, created_at, updated_at
		FROM exercises
		WHERE id = $1
	`

	var exercise models.Exercise
	err := r.db.QueryRow(ctx, query, exerciseID).Scan(
		&exercise.ID, &exercise.Name, &exercise.Sets, &exercise.Reps,
		&exercise.Weight, &exercise.WorkoutID, &exercise.MovementID, &exercise.CreatedAt, &exercise.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get exercise: %w", err)
	}

	return &exercise, nil
}

/**
 * DeleteExercise removes an exercise from the database
 *
 * Args:
 * - ctx: Context for the operation
 * - id: ID of the exercise to delete
 *
 * Returns:
 * - error: Database error if any
 */
func (r *WorkoutRepository) DeleteExercise(ctx context.Context, userID, id string) error {
	query := `DELETE FROM exercises WHERE id = $1 AND workout_id IN (SELECT id FROM workouts WHERE user_id = $2)`
	_, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete exercise: %w", err)
	}
	return nil
}

/**
 * GetWorkoutTemplates returns all available workout templates
 *
 * Returns the predefined workout templates.
 *
 * Args:
 * - ctx: Context for the operation
 *
 * Returns:
 * - []*models.WorkoutTemplate: List of workout templates
 * - error: Database error if any
 */
func (r *WorkoutRepository) GetWorkoutTemplates(ctx context.Context) ([]*models.WorkoutTemplate, error) {
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
		// Chest
		{Name: "Barbell Bench Press", Category: "Chest", DefaultSets: 4, DefaultReps: 8, DefaultWeight: 135},
		{Name: "Dumbbell Bench Press", Category: "Chest", DefaultSets: 3, DefaultReps: 10, DefaultWeight: 40},
		{Name: "Incline Dumbbell Press", Category: "Chest", DefaultSets: 3, DefaultReps: 10, DefaultWeight: 35},
		{Name: "Push-ups", Category: "Chest", DefaultSets: 3, DefaultReps: 15, DefaultWeight: 0},

		// Back
		{Name: "Pull-ups", Category: "Back", DefaultSets: 4, DefaultReps: 8, DefaultWeight: 0},
		{Name: "Barbell Rows", Category: "Back", DefaultSets: 4, DefaultReps: 10, DefaultWeight: 95},
		{Name: "Dumbbell Rows", Category: "Back", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 40},
		{Name: "Lat Pulldowns", Category: "Back", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 80},

		// Shoulders
		{Name: "Overhead Press", Category: "Shoulders", DefaultSets: 3, DefaultReps: 8, DefaultWeight: 65},
		{Name: "Dumbbell Shoulder Press", Category: "Shoulders", DefaultSets: 3, DefaultReps: 10, DefaultWeight: 30},
		{Name: "Lateral Raises", Category: "Shoulders", DefaultSets: 3, DefaultReps: 15, DefaultWeight: 15},
		{Name: "Front Raises", Category: "Shoulders", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 15},

		// Arms
		{Name: "Bicep Curls", Category: "Arms", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 25},
		{Name: "Hammer Curls", Category: "Arms", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 25},
		{Name: "Tricep Pushdowns", Category: "Arms", DefaultSets: 3, DefaultReps: 15, DefaultWeight: 40},
		{Name: "Tricep Dips", Category: "Arms", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 0},

		// Legs
		{Name: "Barbell Squats", Category: "Legs", DefaultSets: 4, DefaultReps: 8, DefaultWeight: 135},
		{Name: "Deadlifts", Category: "Legs", DefaultSets: 4, DefaultReps: 5, DefaultWeight: 135},
		{Name: "Leg Press", Category: "Legs", DefaultSets: 3, DefaultReps: 10, DefaultWeight: 180},
		{Name: "Lunges", Category: "Legs", DefaultSets: 3, DefaultReps: 12, DefaultWeight: 0},

		// Core
		{Name: "Plank", Category: "Core", DefaultSets: 3, DefaultReps: 30, DefaultWeight: 0},
		{Name: "Crunches", Category: "Core", DefaultSets: 3, DefaultReps: 20, DefaultWeight: 0},
		{Name: "Russian Twists", Category: "Core", DefaultSets: 3, DefaultReps: 20, DefaultWeight: 0},
		{Name: "Leg Raises", Category: "Core", DefaultSets: 3, DefaultReps: 15, DefaultWeight: 0},

		// Cardio
		{Name: "Running", Category: "Cardio", DefaultSets: 1, DefaultReps: 20, DefaultWeight: 0},
		{Name: "Cycling", Category: "Cardio", DefaultSets: 1, DefaultReps: 30, DefaultWeight: 0},
		{Name: "Jump Rope", Category: "Cardio", DefaultSets: 5, DefaultReps: 100, DefaultWeight: 0},
		{Name: "Burpees", Category: "Cardio", DefaultSets: 3, DefaultReps: 10, DefaultWeight: 0},
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
func (r *WorkoutRepository) CreateWorkoutFromTemplate(ctx context.Context, userID, templateID string, name string) (*models.Workout, error) {
	templates := r.getPredefinedTemplates()
	var template *models.WorkoutTemplate

	for _, t := range templates {
		if t.ID == templateID {
			template = t
			break
		}
	}

	if template == nil {
		return nil, fmt.Errorf("%w: %s", ErrTemplateNotFound, templateID)
	}

	// Create the workout
	workout, err := r.CreateWorkout(ctx, userID, name)
	if err != nil {
		return nil, err
	}

	// Add exercises from template
	for _, exercise := range template.Exercises {
		exercise.WorkoutID = workout.ID
		err = r.CreateExercise(ctx, userID, &exercise)
		if err != nil {
			return nil, fmt.Errorf("failed to create exercise %s: %w", exercise.Name, err)
		}
	}

	return workout, nil
}

/**
 * CreateDinoGameScore creates a new dino game score in the database
 */
func (r *WorkoutRepository) CreateDinoGameScore(ctx context.Context, userID string, score int) (*models.DinoGameScore, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO dino_game_scores (id, user_id, score, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, score, created_at
	`

	var dinoScore models.DinoGameScore
	err := r.db.QueryRow(ctx, query, id, userID, score, now).Scan(
		&dinoScore.ID, &dinoScore.Score, &dinoScore.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create dino game score: %w", err)
	}

	return &dinoScore, nil
}

/**
 * GetDinoGameHighScore retrieves the highest score from the dino game
 */
func (r *WorkoutRepository) GetDinoGameHighScore(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COALESCE(MAX(score), 0)
		FROM dino_game_scores
		WHERE user_id = $1
	`

	var highScore int
	err := r.db.QueryRow(ctx, query, userID).Scan(&highScore)
	if err != nil {
		return 0, fmt.Errorf("failed to get high score: %w", err)
	}

	return highScore, nil
}
