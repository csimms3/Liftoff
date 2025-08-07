package models

import (
	"time"
)

type Workout struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}

type Exercise struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Sets      int       `json:"sets" db:"sets"`
	Reps      int       `json:"reps" db:"reps"`
	Weight    float64   `json:"weight" db:"weight"`
	WorkoutID string    `json:"workoutId" db:"workout_id"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
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
