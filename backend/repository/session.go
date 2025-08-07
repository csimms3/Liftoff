package repository

import (
	"context"
	"fmt"
	"time"

	"liftoff/backend/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

// WorkoutSession operations
func (r *SessionRepository) CreateSession(ctx context.Context, workoutID string) (*models.WorkoutSession, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO workout_sessions (id, workout_id, started_at, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, workout_id, started_at, ended_at, is_active, created_at, updated_at
	`

	var session models.WorkoutSession
	err := r.db.QueryRow(ctx, query, id, workoutID, now, true, now, now).Scan(
		&session.ID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
		&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &session, nil
}

func (r *SessionRepository) GetActiveSession(ctx context.Context) (*models.WorkoutSession, error) {
	query := `
		SELECT id, workout_id, started_at, ended_at, is_active, created_at, updated_at
		FROM workout_sessions
		WHERE is_active = true
		ORDER BY started_at DESC
		LIMIT 1
	`

	var session models.WorkoutSession
	err := r.db.QueryRow(ctx, query).Scan(
		&session.ID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
		&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}

	return &session, nil
}

func (r *SessionRepository) GetSession(ctx context.Context, id string) (*models.WorkoutSession, error) {
	query := `
		SELECT id, workout_id, started_at, ended_at, is_active, created_at, updated_at
		FROM workout_sessions
		WHERE id = $1
	`

	var session models.WorkoutSession
	err := r.db.QueryRow(ctx, query, id).Scan(
		&session.ID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
		&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

func (r *SessionRepository) EndSession(ctx context.Context, id string) (*models.WorkoutSession, error) {
	query := `
		UPDATE workout_sessions
		SET ended_at = $2, is_active = false, updated_at = $3
		WHERE id = $1
		RETURNING id, workout_id, started_at, ended_at, is_active, created_at, updated_at
	`

	var session models.WorkoutSession
	err := r.db.QueryRow(ctx, query, id, time.Now(), time.Now()).Scan(
		&session.ID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
		&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to end session: %w", err)
	}

	return &session, nil
}

func (r *SessionRepository) GetSessions(ctx context.Context) ([]*models.WorkoutSession, error) {
	query := `
		SELECT id, workout_id, started_at, ended_at, is_active, created_at, updated_at
		FROM workout_sessions
		ORDER BY started_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.WorkoutSession
	for rows.Next() {
		var session models.WorkoutSession
		err := rows.Scan(
			&session.ID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
			&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// SessionExercise operations
func (r *SessionRepository) CreateSessionExercise(ctx context.Context, sessionID, exerciseID string) (*models.SessionExercise, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO session_exercises (id, session_id, exercise_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, session_id, exercise_id, created_at, updated_at
	`

	var sessionExercise models.SessionExercise
	err := r.db.QueryRow(ctx, query, id, sessionID, exerciseID, now, now).Scan(
		&sessionExercise.ID, &sessionExercise.SessionID, &sessionExercise.ExerciseID,
		&sessionExercise.CreatedAt, &sessionExercise.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session exercise: %w", err)
	}

	return &sessionExercise, nil
}

func (r *SessionRepository) GetSessionExercises(ctx context.Context, sessionID string) ([]*models.SessionExercise, error) {
	query := `
		SELECT id, session_id, exercise_id, created_at, updated_at
		FROM session_exercises
		WHERE session_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session exercises: %w", err)
	}
	defer rows.Close()

	var sessionExercises []*models.SessionExercise
	for rows.Next() {
		var sessionExercise models.SessionExercise
		err := rows.Scan(
			&sessionExercise.ID, &sessionExercise.SessionID, &sessionExercise.ExerciseID,
			&sessionExercise.CreatedAt, &sessionExercise.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session exercise: %w", err)
		}
		sessionExercises = append(sessionExercises, &sessionExercise)
	}

	return sessionExercises, nil
}

// ExerciseSet operations
func (r *SessionRepository) CreateExerciseSet(ctx context.Context, set *models.ExerciseSet) error {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO exercise_sets (id, session_exercise_id, reps, weight, completed, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(ctx, query, id, set.SessionExerciseID, set.Reps, set.Weight, set.Completed, set.Notes, now, now)
	if err != nil {
		return fmt.Errorf("failed to create exercise set: %w", err)
	}

	set.ID = id
	set.CreatedAt = now
	set.UpdatedAt = now
	return nil
}

func (r *SessionRepository) GetExerciseSets(ctx context.Context, sessionExerciseID string) ([]*models.ExerciseSet, error) {
	query := `
		SELECT id, session_exercise_id, reps, weight, completed, notes, created_at, updated_at
		FROM exercise_sets
		WHERE session_exercise_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, query, sessionExerciseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get exercise sets: %w", err)
	}
	defer rows.Close()

	var sets []*models.ExerciseSet
	for rows.Next() {
		var set models.ExerciseSet
		err := rows.Scan(
			&set.ID, &set.SessionExerciseID, &set.Reps, &set.Weight,
			&set.Completed, &set.Notes, &set.CreatedAt, &set.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan exercise set: %w", err)
		}
		sets = append(sets, &set)
	}

	return sets, nil
}

func (r *SessionRepository) UpdateExerciseSet(ctx context.Context, set *models.ExerciseSet) error {
	query := `
		UPDATE exercise_sets
		SET reps = $2, weight = $3, completed = $4, notes = $5, updated_at = $6
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, set.ID, set.Reps, set.Weight, set.Completed, set.Notes, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update exercise set: %w", err)
	}

	return nil
}
