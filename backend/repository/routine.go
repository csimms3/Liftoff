package repository

import (
	"context"
	"fmt"
	"time"

	"liftoff/backend/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RoutineTemplateWorkout defines a workout within a routine template
type RoutineTemplateWorkout struct {
	Name      string
	Exercises []models.Exercise
}

// RoutineTemplate defines a predefined multi-workout routine
type RoutineTemplate struct {
	ID          string
	Name        string
	Description string
	Workouts    []RoutineTemplateWorkout
}

type RoutineRepository struct {
	db      *pgxpool.Pool
	workout *WorkoutRepository
}

func NewRoutineRepository(db *pgxpool.Pool, workout *WorkoutRepository) *RoutineRepository {
	return &RoutineRepository{db: db, workout: workout}
}

// CreateRoutine creates a routine with the given workouts in order, in one
// transaction. Every workout must belong to userID (ErrNotFound otherwise).
func (r *RoutineRepository) CreateRoutine(ctx context.Context, userID, name, description string, workoutIDs []string) (*models.Routine, error) {
	id := uuid.New().String()
	err := withTx(ctx, r.db, func(tx tx) error {
		if err := checkWorkoutsOwned(ctx, tx, userID, workoutIDs); err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Exec(ctx, `INSERT INTO routines (id, user_id, name, description, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)`, id, userID, name, description, now, now); err != nil {
			return fmt.Errorf("create routine: %w", err)
		}
		return replaceRoutineWorkouts(ctx, tx, id, workoutIDs)
	})
	if err != nil {
		return nil, err
	}
	return r.GetRoutine(ctx, userID, id)
}

// checkWorkoutsOwned returns ErrNotFound unless every id is one of userID's workouts.
func checkWorkoutsOwned(ctx context.Context, tx tx, userID string, workoutIDs []string) error {
	for _, wid := range workoutIDs {
		var one int
		if err := tx.QueryRow(ctx, `SELECT 1 FROM workouts WHERE id = $1 AND user_id = $2`, []any{wid, userID}, &one); err != nil {
			return err
		}
	}
	return nil
}

// replaceRoutineWorkouts sets the routine's workouts to workoutIDs, in order.
func replaceRoutineWorkouts(ctx context.Context, tx tx, routineID string, workoutIDs []string) error {
	if err := tx.Exec(ctx, `DELETE FROM routine_workouts WHERE routine_id = $1`, routineID); err != nil {
		return fmt.Errorf("clear routine workouts: %w", err)
	}
	now := time.Now()
	for i, wid := range workoutIDs {
		if err := tx.Exec(ctx, `INSERT INTO routine_workouts (id, routine_id, workout_id, slot_order, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)`, uuid.New().String(), routineID, wid, i+1, now, now); err != nil {
			return fmt.Errorf("add routine workout: %w", err)
		}
	}
	return nil
}

func (r *RoutineRepository) GetRoutines(ctx context.Context, userID string) ([]*models.Routine, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, name, description, created_at, updated_at
		FROM routines WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var routines []*models.Routine
	for rows.Next() {
		var routine models.Routine
		if err := rows.Scan(&routine.ID, &routine.UserID, &routine.Name, &routine.Description, &routine.CreatedAt, &routine.UpdatedAt); err != nil {
			return nil, err
		}
		routines = append(routines, &routine)
	}
	for _, routine := range routines {
		routine.Workouts, _ = r.getRoutineWorkouts(ctx, routine.ID)
		for _, rw := range routine.Workouts {
			rw.Workout, _ = r.workout.GetWorkout(ctx, userID, rw.WorkoutID)
		}
	}
	return routines, nil
}

func (r *RoutineRepository) getRoutineWorkouts(ctx context.Context, routineID string) ([]*models.RoutineWorkout, error) {
	rows, err := r.db.Query(ctx, `
		SELECT rw.id, rw.routine_id, rw.workout_id, rw.slot_order, rw.created_at, rw.updated_at
		FROM routine_workouts rw WHERE rw.routine_id = $1 ORDER BY rw.slot_order
	`, routineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.RoutineWorkout
	for rows.Next() {
		var rw models.RoutineWorkout
		if err := rows.Scan(&rw.ID, &rw.RoutineID, &rw.WorkoutID, &rw.SlotOrder, &rw.CreatedAt, &rw.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &rw)
	}
	return list, nil
}

func (r *RoutineRepository) GetRoutine(ctx context.Context, userID, id string) (*models.Routine, error) {
	var routine models.Routine
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, name, description, created_at, updated_at
		FROM routines WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(&routine.ID, &routine.UserID, &routine.Name, &routine.Description, &routine.CreatedAt, &routine.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("routine not found: %w", err)
	}
	routine.Workouts, err = r.getRoutineWorkouts(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, rw := range routine.Workouts {
		rw.Workout, _ = r.workout.GetWorkout(ctx, userID, rw.WorkoutID)
	}
	return &routine, nil
}

// UpdateRoutine sets the routine's name and description and, when workoutIDs is
// non-nil, replaces its workouts, all in one transaction. The routine and every
// workout must belong to userID (ErrNotFound otherwise).
func (r *RoutineRepository) UpdateRoutine(ctx context.Context, userID, id, name, description string, workoutIDs []string) error {
	return withTx(ctx, r.db, func(tx tx) error {
		var one int
		if err := tx.QueryRow(ctx, `SELECT 1 FROM routines WHERE id = $1 AND user_id = $2`, []any{id, userID}, &one); err != nil {
			return err
		}
		if err := tx.Exec(ctx, `UPDATE routines SET name = $1, description = $2, updated_at = $3 WHERE id = $4 AND user_id = $5`,
			name, description, time.Now(), id, userID); err != nil {
			return fmt.Errorf("update routine: %w", err)
		}
		if workoutIDs == nil {
			return nil
		}
		if err := checkWorkoutsOwned(ctx, tx, userID, workoutIDs); err != nil {
			return err
		}
		return replaceRoutineWorkouts(ctx, tx, id, workoutIDs)
	})
}

func (r *RoutineRepository) DeleteRoutine(ctx context.Context, userID, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM routines WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

func (r *RoutineRepository) GetRoutineTemplates() []RoutineTemplate {
	return getRoutineTemplates()
}

func (r *RoutineRepository) CreateFromTemplate(ctx context.Context, userID, templateID string, routineName string) (*models.Routine, error) {
	templates := getRoutineTemplates()
	var tpl *RoutineTemplate
	for i := range templates {
		if templates[i].ID == templateID {
			tpl = &templates[i]
			break
		}
	}
	if tpl == nil {
		return nil, fmt.Errorf("%w: %s", ErrTemplateNotFound, templateID)
	}
	name := routineName
	if name == "" {
		name = tpl.Name
	}

	var workoutIDs []string
	for _, w := range tpl.Workouts {
		workout, err := r.workout.CreateWorkout(ctx, userID, w.Name)
		if err != nil {
			return nil, fmt.Errorf("create workout %s: %w", w.Name, err)
		}
		for _, ex := range w.Exercises {
			ex.WorkoutID = workout.ID
			if err := r.workout.CreateExercise(ctx, userID, &ex); err != nil {
				return nil, fmt.Errorf("create exercise %s: %w", ex.Name, err)
			}
		}
		workoutIDs = append(workoutIDs, workout.ID)
	}

	return r.CreateRoutine(ctx, userID, name, tpl.Description, workoutIDs)
}

func getRoutineTemplates() []RoutineTemplate {
	return []RoutineTemplate{
		{
			ID:          "push-pull-legs",
			Name:        "Push Pull Legs",
			Description: "Classic 3-day split: Push (chest, shoulders, triceps), Pull (back, biceps), Legs",
			Workouts: []RoutineTemplateWorkout{
				{
					Name: "Push",
					Exercises: []models.Exercise{
						{Name: "Barbell Bench Press", Sets: 4, Reps: 8, Weight: 135},
						{Name: "Overhead Press", Sets: 3, Reps: 8, Weight: 65},
						{Name: "Incline Dumbbell Press", Sets: 3, Reps: 10, Weight: 35},
						{Name: "Lateral Raises", Sets: 3, Reps: 15, Weight: 15},
						{Name: "Tricep Pushdowns", Sets: 3, Reps: 15, Weight: 40},
					},
				},
				{
					Name: "Pull",
					Exercises: []models.Exercise{
						{Name: "Pull-ups", Sets: 4, Reps: 8, Weight: 0},
						{Name: "Barbell Rows", Sets: 4, Reps: 10, Weight: 95},
						{Name: "Lat Pulldowns", Sets: 3, Reps: 12, Weight: 80},
						{Name: "Dumbbell Rows", Sets: 3, Reps: 12, Weight: 40},
						{Name: "Bicep Curls", Sets: 3, Reps: 12, Weight: 25},
					},
				},
				{
					Name: "Legs",
					Exercises: []models.Exercise{
						{Name: "Barbell Squats", Sets: 4, Reps: 8, Weight: 135},
						{Name: "Deadlifts", Sets: 4, Reps: 5, Weight: 135},
						{Name: "Leg Press", Sets: 3, Reps: 10, Weight: 180},
						{Name: "Lunges", Sets: 3, Reps: 12, Weight: 0},
						{Name: "Leg Raises", Sets: 3, Reps: 15, Weight: 0},
					},
				},
			},
		},
		{
			ID:          "upper-lower",
			Name:        "Upper Lower",
			Description: "2-day split: Upper body and Lower body",
			Workouts: []RoutineTemplateWorkout{
				{
					Name: "Upper",
					Exercises: []models.Exercise{
						{Name: "Barbell Bench Press", Sets: 4, Reps: 8, Weight: 135},
						{Name: "Barbell Rows", Sets: 4, Reps: 10, Weight: 95},
						{Name: "Overhead Press", Sets: 3, Reps: 8, Weight: 65},
						{Name: "Pull-ups", Sets: 3, Reps: 8, Weight: 0},
						{Name: "Bicep Curls", Sets: 3, Reps: 12, Weight: 25},
						{Name: "Tricep Pushdowns", Sets: 3, Reps: 15, Weight: 40},
					},
				},
				{
					Name: "Lower",
					Exercises: []models.Exercise{
						{Name: "Barbell Squats", Sets: 4, Reps: 8, Weight: 135},
						{Name: "Deadlifts", Sets: 4, Reps: 5, Weight: 135},
						{Name: "Leg Press", Sets: 3, Reps: 10, Weight: 180},
						{Name: "Lunges", Sets: 3, Reps: 12, Weight: 0},
						{Name: "Leg Raises", Sets: 3, Reps: 15, Weight: 0},
					},
				},
			},
		},
		{
			ID:          "upper-lower-4day",
			Name:        "Upper Lower 4-Day",
			Description: "4-day variation: Upper A, Lower A, Upper B, Lower B with different exercises",
			Workouts: []RoutineTemplateWorkout{
				{
					Name: "Upper A",
					Exercises: []models.Exercise{
						{Name: "Barbell Bench Press", Sets: 4, Reps: 8, Weight: 135},
						{Name: "Barbell Rows", Sets: 4, Reps: 10, Weight: 95},
						{Name: "Overhead Press", Sets: 3, Reps: 8, Weight: 65},
						{Name: "Lat Pulldowns", Sets: 3, Reps: 12, Weight: 80},
						{Name: "Bicep Curls", Sets: 3, Reps: 12, Weight: 25},
						{Name: "Tricep Pushdowns", Sets: 3, Reps: 15, Weight: 40},
					},
				},
				{
					Name: "Lower A",
					Exercises: []models.Exercise{
						{Name: "Barbell Squats", Sets: 4, Reps: 8, Weight: 135},
						{Name: "Romanian Deadlifts", Sets: 3, Reps: 10, Weight: 95},
						{Name: "Leg Press", Sets: 3, Reps: 10, Weight: 180},
						{Name: "Leg Curls", Sets: 3, Reps: 12, Weight: 0},
						{Name: "Calf Raises", Sets: 3, Reps: 15, Weight: 0},
					},
				},
				{
					Name: "Upper B",
					Exercises: []models.Exercise{
						{Name: "Incline Dumbbell Press", Sets: 4, Reps: 10, Weight: 35},
						{Name: "Dumbbell Rows", Sets: 4, Reps: 12, Weight: 40},
						{Name: "Dumbbell Shoulder Press", Sets: 3, Reps: 10, Weight: 30},
						{Name: "Pull-ups", Sets: 3, Reps: 8, Weight: 0},
						{Name: "Hammer Curls", Sets: 3, Reps: 12, Weight: 25},
						{Name: "Tricep Dips", Sets: 3, Reps: 12, Weight: 0},
					},
				},
				{
					Name: "Lower B",
					Exercises: []models.Exercise{
						{Name: "Deadlifts", Sets: 4, Reps: 5, Weight: 135},
						{Name: "Front Squats", Sets: 3, Reps: 8, Weight: 95},
						{Name: "Leg Press", Sets: 3, Reps: 12, Weight: 180},
						{Name: "Lunges", Sets: 3, Reps: 12, Weight: 0},
						{Name: "Leg Raises", Sets: 3, Reps: 15, Weight: 0},
					},
				},
			},
		},
		{
			ID:          "full-body",
			Name:        "Full Body",
			Description: "Single full-body workout for 3x per week",
			Workouts: []RoutineTemplateWorkout{
				{
					Name: "Full Body",
					Exercises: []models.Exercise{
						{Name: "Barbell Squats", Sets: 3, Reps: 8, Weight: 135},
						{Name: "Barbell Bench Press", Sets: 3, Reps: 8, Weight: 135},
						{Name: "Barbell Rows", Sets: 3, Reps: 10, Weight: 95},
						{Name: "Overhead Press", Sets: 3, Reps: 8, Weight: 65},
						{Name: "Deadlifts", Sets: 3, Reps: 5, Weight: 135},
						{Name: "Plank", Sets: 3, Reps: 30, Weight: 0},
					},
				},
			},
		},
	}
}
