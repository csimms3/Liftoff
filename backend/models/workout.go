package models

import (
	"time"
)

// Workout types for categorization
const (
	WorkoutTypeStrength = "strength"
	WorkoutTypeCardio   = "cardio"
	WorkoutTypeFlexibility = "flexibility"
	WorkoutTypeHIIT     = "hiit"
	WorkoutTypeEndurance = "endurance"
	WorkoutTypePower    = "power"
)

// Workout represents a workout plan
type Workout struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Type      string    `json:"type" db:"type"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// WorkoutTemplate represents a predefined workout template
type WorkoutTemplate struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Type        string    `json:"type" db:"type"`
	Description string    `json:"description" db:"description"`
	Difficulty  string    `json:"difficulty" db:"difficulty"`
	Duration    int       `json:"duration" db:"duration"` // in minutes
	Exercises   []Exercise `json:"exercises" db:"-"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Exercise represents an exercise within a workout
type Exercise struct {
	ID         string    `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	Sets       int       `json:"sets" db:"sets"`
	Reps       int       `json:"reps" db:"reps"`
	Weight     float64   `json:"weight" db:"weight"`
	WorkoutID  string    `json:"workout_id" db:"workout_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// ExerciseTemplate represents a predefined exercise template
type ExerciseTemplate struct {
	Name         string  `json:"name" db:"name"`
	Category     string  `json:"category" db:"category"`
	DefaultSets  int     `json:"default_sets" db:"default_sets"`
	DefaultReps  int     `json:"default_reps" db:"default_reps"`
	DefaultWeight float64 `json:"default_weight" db:"default_weight"`
}

type WorkoutSession struct {
	ID        string     `json:"id" db:"id"`
	WorkoutID string     `json:"workoutId" db:"workout_id"`
	StartedAt time.Time  `json:"startedAt" db:"started_at"`
	EndedAt   *time.Time `json:"endedAt" db:"ended_at"`
	IsActive  bool       `json:"isActive" db:"is_active"`
	CreatedAt time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time  `json:"updatedAt" db:"updated_at"`
}

type SessionExercise struct {
	ID         string    `json:"id" db:"id"`
	SessionID  string    `json:"sessionId" db:"session_id"`
	ExerciseID string    `json:"exerciseId" db:"exercise_id"`
	CreatedAt  time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt  time.Time `json:"updatedAt" db:"updated_at"`
}

type ExerciseSet struct {
	ID                string    `json:"id" db:"id"`
	SessionExerciseID string    `json:"sessionExerciseId" db:"session_exercise_id"`
	Reps              int       `json:"reps" db:"reps"`
	Weight            float64   `json:"weight" db:"weight"`
	Completed         bool      `json:"completed" db:"completed"`
	Notes             *string   `json:"notes" db:"notes"`
	CreatedAt         time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time `json:"updatedAt" db:"updated_at"`
}
