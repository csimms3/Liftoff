package models

import "time"

// Routine represents a multi-workout program (e.g., Push Pull Legs)
type Routine struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"-" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	Workouts    []*RoutineWorkout `json:"workouts" db:"-"`
}

// RoutineWorkout links a workout to a routine with ordering
type RoutineWorkout struct {
	ID         string    `json:"id" db:"id"`
	RoutineID  string    `json:"routine_id" db:"routine_id"`
	WorkoutID  string    `json:"workout_id" db:"workout_id"`
	SlotOrder  int       `json:"slot_order" db:"slot_order"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	Workout    *Workout  `json:"workout" db:"-"`
}
