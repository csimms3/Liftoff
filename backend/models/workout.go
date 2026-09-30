package models

import (
	"time"
)

// Workout types for categorization
const (
	WorkoutTypeStrength    = "strength"
	WorkoutTypeCardio      = "cardio"
	WorkoutTypeFlexibility = "flexibility"
	WorkoutTypeHIIT        = "hiit"
	WorkoutTypeEndurance   = "endurance"
	WorkoutTypePower       = "power"
)

// Workout represents a workout plan with exercises
type Workout struct {
	ID     string `json:"id" db:"id"`
	UserID string `json:"-" db:"user_id"`
	// RoutineID is the routine the workout belongs to (every workout has exactly one).
	RoutineID string     `json:"routine_id" db:"routine_id"`
	Name      string     `json:"name" db:"name"`
	Type      string     `json:"type" db:"type"`
	Exercises []Exercise `json:"exercises" db:"-"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

// WorkoutTemplate represents a predefined workout template with exercises
type WorkoutTemplate struct {
	ID          string     `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"`
	Type        string     `json:"type" db:"type"`
	Description string     `json:"description" db:"description"`
	Difficulty  string     `json:"difficulty" db:"difficulty"`
	Duration    int        `json:"duration" db:"duration"` // in minutes
	Exercises   []Exercise `json:"exercises" db:"-"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// Exercise represents an exercise within a workout
type Exercise struct {
	ID        string  `json:"id" db:"id"`
	Name      string  `json:"name" db:"name"`
	Sets      int     `json:"sets" db:"sets"`
	Reps      int     `json:"reps" db:"reps"`
	Weight    float64 `json:"weight" db:"weight"`
	WorkoutID string  `json:"workout_id" db:"workout_id"`
	// MovementID is the catalog entry this exercise performs (shared across
	// workouts that use the same exercise name).
	MovementID string    `json:"movement_id" db:"movement_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// Movement is one of a user's exercises as a catalog entry ("Barbell Squat"):
// the identity that history, previous values and progress follow.
type Movement struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"-" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Category    string    `json:"category" db:"category"`
	RestSeconds int       `json:"rest_seconds" db:"rest_seconds"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ExerciseTemplate represents a predefined exercise template for quick addition
type ExerciseTemplate struct {
	Name          string  `json:"name" db:"name"`
	Category      string  `json:"category" db:"category"`
	DefaultSets   int     `json:"default_sets" db:"default_sets"`
	DefaultReps   int     `json:"default_reps" db:"default_reps"`
	DefaultWeight float64 `json:"default_weight" db:"default_weight"`
}

// WorkoutSession represents an active or completed workout session
type WorkoutSession struct {
	ID     string `json:"id" db:"id"`
	UserID string `json:"-" db:"user_id"`
	// WorkoutID is empty once the workout has been deleted; WorkoutName keeps the
	// name the session was performed under.
	WorkoutID   string             `json:"workout_id" db:"workout_id"`
	WorkoutName string             `json:"workout_name" db:"workout_name"`
	Workout     *Workout           `json:"workout" db:"-"`
	StartedAt   time.Time          `json:"started_at" db:"started_at"`
	EndedAt     *time.Time         `json:"ended_at" db:"ended_at"`
	IsActive    bool               `json:"is_active" db:"is_active"`
	Exercises   []*SessionExercise `json:"exercises" db:"-"`
	CreatedAt   time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" db:"updated_at"`
}

// SessionExercise represents an exercise performed during a workout session
type SessionExercise struct {
	ID        string `json:"id" db:"id"`
	SessionID string `json:"session_id" db:"session_id"`
	// ExerciseID is the workout's exercise it came from; empty once that exercise
	// has been removed from the workout. Name is kept as performed.
	ExerciseID string `json:"exercise_id" db:"exercise_id"`
	MovementID string `json:"movement_id" db:"movement_id"`
	Name       string `json:"name" db:"name"`
	// RestSeconds is the movement's rest timer (0 = off), remembered per exercise.
	RestSeconds int            `json:"rest_seconds" db:"-"`
	Exercise    *Exercise      `json:"exercise" db:"-"`
	Sets        []*ExerciseSet `json:"sets" db:"-"`
	// Previous holds the completed sets for this exercise from the user's last
	// session that logged it, in order; empty when there is none.
	Previous  []*ExerciseSet `json:"previous" db:"-"`
	CreatedAt time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt time.Time      `json:"updated_at" db:"updated_at"`
}

// ExerciseSet represents a single set of an exercise during a session
type ExerciseSet struct {
	ID                string    `json:"id" db:"id"`
	SessionExerciseID string    `json:"session_exercise_id" db:"session_exercise_id"`
	Reps              int       `json:"reps" db:"reps"`
	Weight            float64   `json:"weight" db:"weight"`
	Completed         bool      `json:"completed" db:"completed"`
	Notes             *string   `json:"notes" db:"notes"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

// DinoGameScore represents a score from the Dino Game easter egg
type DinoGameScore struct {
	ID        string    `json:"id" db:"id"`
	Score     int       `json:"score" db:"score"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
