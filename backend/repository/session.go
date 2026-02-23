package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"liftoff/backend/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepository struct {
	db        *pgxpool.Pool
	sqlite    *sql.DB
	useSQLite bool
}

func NewSessionRepository(db *pgxpool.Pool, sqlite *sql.DB, useSQLite bool) *SessionRepository {
	if useSQLite {
		return &SessionRepository{db: nil, sqlite: sqlite, useSQLite: true}
	}
	return &SessionRepository{db: db, sqlite: nil, useSQLite: false}
}

// WorkoutSession operations
func (r *SessionRepository) CreateSession(ctx context.Context, userID, workoutID string) (*models.WorkoutSession, error) {
	if r.useSQLite {
		return r.createSessionSQLite(ctx, userID, workoutID)
	}
	return r.createSessionPostgres(ctx, userID, workoutID)
}

// CreateSessionWithExercises creates a session and initializes all exercises with sets
func (r *SessionRepository) CreateSessionWithExercises(ctx context.Context, userID, workoutID string) (*models.WorkoutSession, error) {
	// Create the session first
	session, err := r.CreateSession(ctx, userID, workoutID)
	if err != nil {
		return nil, err
	}

	// Get the workout to access its exercises (verify ownership)
	workoutRepo := NewWorkoutRepository(r.db, r.sqlite, r.useSQLite)
	workout, err := workoutRepo.GetWorkout(ctx, userID, workoutID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workout: %w", err)
	}

	// Create session exercises and sets for each exercise
	for _, exercise := range workout.Exercises {
		// Create session exercise (no userID check - we're creating)
		sessionExercise, err := r.CreateSessionExercise(ctx, "", session.ID, exercise.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to create session exercise: %w", err)
		}

		// Create sets for this exercise (no userID - internal create)
		for i := 0; i < exercise.Sets; i++ {
			set := &models.ExerciseSet{
				SessionExerciseID: sessionExercise.ID,
				Reps:              exercise.Reps,
				Weight:            exercise.Weight,
				Completed:         false,
			}
			err = r.CreateExerciseSet(ctx, "", set)
			if err != nil {
				return nil, fmt.Errorf("failed to create exercise set: %w", err)
			}
		}
	}

	// Return the session with exercises populated
	return r.GetActiveSessionWithExercises(ctx, userID)
}

// GetActiveSessionWithExercises returns the active session with all exercises and sets populated
func (r *SessionRepository) GetActiveSessionWithExercises(ctx context.Context, userID string) (*models.WorkoutSession, error) {
	session, err := r.GetActiveSession(ctx, userID)
	if err != nil || session == nil {
		return nil, err
	}

	// Get session exercises
	sessionExercises, err := r.GetSessionExercises(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session exercises: %w", err)
	}

	// Populate exercises with sets and exercise details
	for _, se := range sessionExercises {
		// Get exercise details
		workoutRepo := NewWorkoutRepository(r.db, r.sqlite, r.useSQLite)
		exercise, err := workoutRepo.GetExercise(ctx, se.ExerciseID)
		if err != nil {
			return nil, fmt.Errorf("failed to get exercise: %w", err)
		}
		se.Exercise = exercise
		
		// Get sets for this exercise
		sets, err := r.GetExerciseSets(ctx, se.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get exercise sets: %w", err)
		}
		se.Sets = sets
	}

	// Get workout details (session already filtered by user)
	workoutRepo := NewWorkoutRepository(r.db, r.sqlite, r.useSQLite)
	workout, err := workoutRepo.GetWorkout(ctx, userID, session.WorkoutID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workout: %w", err)
	}

	// Create a session with exercises populated
	sessionWithExercises := &models.WorkoutSession{
		ID:        session.ID,
		WorkoutID: session.WorkoutID,
		StartedAt: session.StartedAt,
		EndedAt:   session.EndedAt,
		IsActive:  session.IsActive,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
		Workout:   workout,
		Exercises: sessionExercises,
	}

	return sessionWithExercises, nil
}

// GetCompletedSessions returns all completed workout sessions for the user
func (r *SessionRepository) GetCompletedSessions(ctx context.Context, userID string) ([]*models.WorkoutSession, error) {
	if r.useSQLite {
		return r.getCompletedSessionsSQLite(ctx, userID)
	}
	return r.getCompletedSessionsPostgres(ctx, userID)
}

func (r *SessionRepository) getCompletedSessionsPostgres(ctx context.Context, userID string) ([]*models.WorkoutSession, error) {
	query := `
		SELECT id, user_id, workout_id, started_at, ended_at, is_active, created_at, updated_at
		FROM workout_sessions
		WHERE user_id = $1 AND is_active = false AND ended_at IS NOT NULL
		ORDER BY ended_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get completed sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.WorkoutSession
	for rows.Next() {
		var session models.WorkoutSession
		err := rows.Scan(
			&session.ID, &session.UserID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
			&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func (r *SessionRepository) getCompletedSessionsSQLite(ctx context.Context, userID string) ([]*models.WorkoutSession, error) {
	query := `
		SELECT id, user_id, workout_id, started_at, ended_at, is_active, created_at, updated_at
		FROM workout_sessions
		WHERE user_id = ? AND is_active = 0 AND ended_at IS NOT NULL
		ORDER BY ended_at DESC
	`

	rows, err := r.sqlite.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get completed sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.WorkoutSession
	for rows.Next() {
		var session models.WorkoutSession
		err := rows.Scan(
			&session.ID, &session.UserID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
			&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func (r *SessionRepository) createSessionPostgres(ctx context.Context, userID, workoutID string) (*models.WorkoutSession, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO workout_sessions (id, user_id, workout_id, started_at, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, workout_id, started_at, ended_at, is_active, created_at, updated_at
	`

	var session models.WorkoutSession
	err := r.db.QueryRow(ctx, query, id, userID, workoutID, now, true, now, now).Scan(
		&session.ID, &session.UserID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
		&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &session, nil
}

func (r *SessionRepository) createSessionSQLite(ctx context.Context, userID, workoutID string) (*models.WorkoutSession, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO workout_sessions (id, user_id, workout_id, started_at, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.sqlite.ExecContext(ctx, query, id, userID, workoutID, now, true, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &models.WorkoutSession{
		ID:        id,
		UserID:    userID,
		WorkoutID: workoutID,
		StartedAt: now,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (r *SessionRepository) GetActiveSession(ctx context.Context, userID string) (*models.WorkoutSession, error) {
	if r.useSQLite {
		return r.getActiveSessionSQLite(ctx, userID)
	}
	return r.getActiveSessionPostgres(ctx, userID)
}

func (r *SessionRepository) getActiveSessionPostgres(ctx context.Context, userID string) (*models.WorkoutSession, error) {
	query := `
		SELECT id, user_id, workout_id, started_at, ended_at, is_active, created_at, updated_at
		FROM workout_sessions
		WHERE user_id = $1 AND is_active = true
		ORDER BY started_at DESC
		LIMIT 1
	`

	var session models.WorkoutSession
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&session.ID, &session.UserID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
		&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No active session found
		}
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}

	return &session, nil
}

func (r *SessionRepository) getActiveSessionSQLite(ctx context.Context, userID string) (*models.WorkoutSession, error) {
	query := `
		SELECT id, user_id, workout_id, started_at, ended_at, is_active, created_at, updated_at
		FROM workout_sessions
		WHERE user_id = ? AND is_active = 1
		ORDER BY started_at DESC
		LIMIT 1
	`

	var session models.WorkoutSession
	err := r.sqlite.QueryRowContext(ctx, query, userID).Scan(
		&session.ID, &session.UserID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
		&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No active session found
		}
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}

	return &session, nil
}

func (r *SessionRepository) GetSession(ctx context.Context, id string) (*models.WorkoutSession, error) {
	if r.useSQLite {
		return r.getSessionSQLite(ctx, id)
	}
	return r.getSessionPostgres(ctx, id)
}

func (r *SessionRepository) getSessionPostgres(ctx context.Context, id string) (*models.WorkoutSession, error) {
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

func (r *SessionRepository) getSessionSQLite(ctx context.Context, id string) (*models.WorkoutSession, error) {
	query := `
		SELECT id, workout_id, started_at, ended_at, is_active, created_at, updated_at
		FROM workout_sessions
		WHERE id = ?
	`

	var session models.WorkoutSession
	err := r.sqlite.QueryRowContext(ctx, query, id).Scan(
		&session.ID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
		&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

func (r *SessionRepository) EndSession(ctx context.Context, userID, id string) (*models.WorkoutSession, error) {
	if r.useSQLite {
		return r.endSessionSQLite(ctx, userID, id)
	}
	return r.endSessionPostgres(ctx, userID, id)
}

func (r *SessionRepository) endSessionPostgres(ctx context.Context, userID, id string) (*models.WorkoutSession, error) {
	query := `
		UPDATE workout_sessions
		SET ended_at = $2, is_active = false, updated_at = $3
		WHERE id = $1 AND user_id = $4
		RETURNING id, user_id, workout_id, started_at, ended_at, is_active, created_at, updated_at
	`

	var session models.WorkoutSession
	err := r.db.QueryRow(ctx, query, id, time.Now(), time.Now(), userID).Scan(
		&session.ID, &session.UserID, &session.WorkoutID, &session.StartedAt, &session.EndedAt,
		&session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to end session: %w", err)
	}

	return &session, nil
}

func (r *SessionRepository) endSessionSQLite(ctx context.Context, userID, id string) (*models.WorkoutSession, error) {
	query := `
		UPDATE workout_sessions
		SET ended_at = ?, is_active = 0, updated_at = ?
		WHERE id = ? AND user_id = ?
	`

	result, err := r.sqlite.ExecContext(ctx, query, time.Now(), time.Now(), id, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to end session: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("session not found or access denied")
	}

	// Get the updated session
	return r.getSessionSQLite(ctx, id)
}

func (r *SessionRepository) GetSessions(ctx context.Context) ([]*models.WorkoutSession, error) {
	if r.useSQLite {
		return r.getSessionsSQLite(ctx)
	}
	return r.getSessionsPostgres(ctx)
}

func (r *SessionRepository) getSessionsPostgres(ctx context.Context) ([]*models.WorkoutSession, error) {
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

func (r *SessionRepository) getSessionsSQLite(ctx context.Context) ([]*models.WorkoutSession, error) {
	query := `
		SELECT id, workout_id, started_at, ended_at, is_active, created_at, updated_at
		FROM workout_sessions
		ORDER BY started_at DESC
	`

	rows, err := r.sqlite.QueryContext(ctx, query)
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
func (r *SessionRepository) CreateSessionExercise(ctx context.Context, userID, sessionID, exerciseID string) (*models.SessionExercise, error) {
	// Verify session belongs to user (when userID is provided - skip for internal CreateSessionWithExercises by passing "")
	if userID != "" {
		session, err := r.getSessionForUser(ctx, userID, sessionID)
		if err != nil || session == nil {
			return nil, fmt.Errorf("session not found or access denied")
		}
	}
	if r.useSQLite {
		return r.createSessionExerciseSQLite(ctx, sessionID, exerciseID)
	}
	return r.createSessionExercisePostgres(ctx, sessionID, exerciseID)
}

func (r *SessionRepository) getSessionForUser(ctx context.Context, userID, sessionID string) (*models.WorkoutSession, error) {
	if r.useSQLite {
		return r.getSessionForUserSQLite(ctx, userID, sessionID)
	}
	return r.getSessionForUserPostgres(ctx, userID, sessionID)
}

func (r *SessionRepository) getSessionForUserPostgres(ctx context.Context, userID, sessionID string) (*models.WorkoutSession, error) {
	query := `SELECT id FROM workout_sessions WHERE id = $1 AND user_id = $2`
	var id string
	err := r.db.QueryRow(ctx, query, sessionID, userID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &models.WorkoutSession{ID: id}, nil
}

func (r *SessionRepository) getSessionForUserSQLite(ctx context.Context, userID, sessionID string) (*models.WorkoutSession, error) {
	query := `SELECT id FROM workout_sessions WHERE id = ? AND user_id = ?`
	var id string
	err := r.sqlite.QueryRowContext(ctx, query, sessionID, userID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &models.WorkoutSession{ID: id}, nil
}

func (r *SessionRepository) createSessionExercisePostgres(ctx context.Context, sessionID, exerciseID string) (*models.SessionExercise, error) {
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

func (r *SessionRepository) createSessionExerciseSQLite(ctx context.Context, sessionID, exerciseID string) (*models.SessionExercise, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO session_exercises (id, session_id, exercise_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := r.sqlite.ExecContext(ctx, query, id, sessionID, exerciseID, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create session exercise: %w", err)
	}

	return &models.SessionExercise{
		ID:         id,
		SessionID:  sessionID,
		ExerciseID: exerciseID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (r *SessionRepository) GetSessionExercises(ctx context.Context, sessionID string) ([]*models.SessionExercise, error) {
	if r.useSQLite {
		return r.getSessionExercisesSQLite(ctx, sessionID)
	}
	return r.getSessionExercisesPostgres(ctx, sessionID)
}

func (r *SessionRepository) getSessionExercisesPostgres(ctx context.Context, sessionID string) ([]*models.SessionExercise, error) {
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

func (r *SessionRepository) getSessionExercisesSQLite(ctx context.Context, sessionID string) ([]*models.SessionExercise, error) {
	query := `
		SELECT id, session_id, exercise_id, created_at, updated_at
		FROM session_exercises
		WHERE session_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.sqlite.QueryContext(ctx, query, sessionID)
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
func (r *SessionRepository) CreateExerciseSet(ctx context.Context, userID string, set *models.ExerciseSet) error {
	if userID != "" {
		if !r.verifySessionExerciseAccess(ctx, userID, set.SessionExerciseID) {
			return fmt.Errorf("session exercise not found or access denied")
		}
	}
	if r.useSQLite {
		return r.createExerciseSetSQLite(ctx, set)
	}
	return r.createExerciseSetPostgres(ctx, set)
}

func (r *SessionRepository) getSessionExerciseIDForSet(ctx context.Context, setID string) (string, error) {
	var query string
	if r.useSQLite {
		query = `SELECT session_exercise_id FROM exercise_sets WHERE id = ?`
	} else {
		query = `SELECT session_exercise_id FROM exercise_sets WHERE id = $1`
	}
	var sessionExerciseID string
	var err error
	if r.useSQLite {
		err = r.sqlite.QueryRowContext(ctx, query, setID).Scan(&sessionExerciseID)
	} else {
		err = r.db.QueryRow(ctx, query, setID).Scan(&sessionExerciseID)
	}
	return sessionExerciseID, err
}

func (r *SessionRepository) verifySessionExerciseAccess(ctx context.Context, userID, sessionExerciseID string) bool {
	var query string
	if r.useSQLite {
		query = `SELECT 1 FROM session_exercises se JOIN workout_sessions ws ON se.session_id = ws.id WHERE se.id = ? AND ws.user_id = ?`
	} else {
		query = `SELECT 1 FROM session_exercises se JOIN workout_sessions ws ON se.session_id = ws.id WHERE se.id = $1 AND ws.user_id = $2`
	}
	var result int
	var err error
	if r.useSQLite {
		err = r.sqlite.QueryRowContext(ctx, query, sessionExerciseID, userID).Scan(&result)
	} else {
		err = r.db.QueryRow(ctx, query, sessionExerciseID, userID).Scan(&result)
	}
	return err == nil
}

func (r *SessionRepository) createExerciseSetPostgres(ctx context.Context, set *models.ExerciseSet) error {
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

func (r *SessionRepository) createExerciseSetSQLite(ctx context.Context, set *models.ExerciseSet) error {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO exercise_sets (id, session_exercise_id, reps, weight, completed, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.sqlite.ExecContext(ctx, query, id, set.SessionExerciseID, set.Reps, set.Weight, set.Completed, set.Notes, now, now)
	if err != nil {
		return fmt.Errorf("failed to create exercise set: %w", err)
	}

	set.ID = id
	set.CreatedAt = now
	set.UpdatedAt = now
	return nil
}

func (r *SessionRepository) GetExerciseSets(ctx context.Context, sessionExerciseID string) ([]*models.ExerciseSet, error) {
	if r.useSQLite {
		return r.getExerciseSetsSQLite(ctx, sessionExerciseID)
	}
	return r.getExerciseSetsPostgres(ctx, sessionExerciseID)
}

func (r *SessionRepository) getExerciseSetsPostgres(ctx context.Context, sessionExerciseID string) ([]*models.ExerciseSet, error) {
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

func (r *SessionRepository) getExerciseSetsSQLite(ctx context.Context, sessionExerciseID string) ([]*models.ExerciseSet, error) {
	query := `
		SELECT id, session_exercise_id, reps, weight, completed, notes, created_at, updated_at
		FROM exercise_sets
		WHERE session_exercise_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.sqlite.QueryContext(ctx, query, sessionExerciseID)
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

func (r *SessionRepository) UpdateExerciseSet(ctx context.Context, userID string, set *models.ExerciseSet) error {
	if userID != "" {
		sessionExerciseID := set.SessionExerciseID
		if sessionExerciseID == "" {
			// Fetch from DB to get SessionExerciseID for verification
			seID, err := r.getSessionExerciseIDForSet(ctx, set.ID)
			if err != nil {
				return fmt.Errorf("exercise set not found or access denied")
			}
			sessionExerciseID = seID
			set.SessionExerciseID = seID
		}
		if !r.verifySessionExerciseAccess(ctx, userID, sessionExerciseID) {
			return fmt.Errorf("exercise set not found or access denied")
		}
	}
	if r.useSQLite {
		return r.updateExerciseSetSQLite(ctx, set)
	}
	return r.updateExerciseSetPostgres(ctx, set)
}

func (r *SessionRepository) updateExerciseSetPostgres(ctx context.Context, set *models.ExerciseSet) error {
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

func (r *SessionRepository) updateExerciseSetSQLite(ctx context.Context, set *models.ExerciseSet) error {
	query := `
		UPDATE exercise_sets
		SET reps = ?, weight = ?, completed = ?, notes = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.sqlite.ExecContext(ctx, query, set.Reps, set.Weight, set.Completed, set.Notes, time.Now(), set.ID)
	if err != nil {
		return fmt.Errorf("failed to update exercise set: %w", err)
	}

	return nil
}

func (r *SessionRepository) CompleteExerciseSet(ctx context.Context, userID, sessionExerciseID string, setIndex int) error {
	if userID != "" && !r.verifySessionExerciseAccess(ctx, userID, sessionExerciseID) {
		return fmt.Errorf("session exercise not found or access denied")
	}
	// Get all sets for this session exercise
	sets, err := r.GetExerciseSets(ctx, sessionExerciseID)
	if err != nil {
		return fmt.Errorf("failed to get exercise sets: %w", err)
	}

	// Check if setIndex is valid
	if setIndex < 0 || setIndex >= len(sets) {
		return fmt.Errorf("invalid set index: %d", setIndex)
	}

	// Mark the specified set as completed
	set := sets[setIndex]
	set.Completed = true

	return r.UpdateExerciseSet(ctx, userID, set)
}

func (r *SessionRepository) GetProgressData(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	if r.useSQLite {
		return r.getProgressDataSQLite(ctx, userID)
	}
	return r.getProgressDataPostgres(ctx, userID)
}

func (r *SessionRepository) getProgressDataPostgres(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			e.name as exercise_name,
			DATE(es.created_at) as workout_date,
			MAX(es.weight) as max_weight,
			SUM(es.weight * es.reps) as total_volume
		FROM exercise_sets es
		JOIN session_exercises se ON es.session_exercise_id = se.id
		JOIN workout_sessions ws ON se.session_id = ws.id
		JOIN exercises e ON se.exercise_id = e.id
		WHERE es.completed = true AND ws.user_id = $1
		GROUP BY e.name, DATE(es.created_at)
		ORDER BY workout_date DESC, exercise_name
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get progress data: %w", err)
	}
	defer rows.Close()

	var progress []map[string]interface{}
	for rows.Next() {
		var exerciseName string
		var workoutDate time.Time
		var maxWeight float64
		var totalVolume float64

		err := rows.Scan(&exerciseName, &workoutDate, &maxWeight, &totalVolume)
		if err != nil {
			return nil, fmt.Errorf("failed to scan progress data: %w", err)
		}

		progress = append(progress, map[string]interface{}{
			"exerciseName": exerciseName,
			"date":         workoutDate.Format("2006-01-02"),
			"maxWeight":    maxWeight,
			"totalVolume":  totalVolume,
		})
	}

	return progress, nil
}

func (r *SessionRepository) getProgressDataSQLite(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			e.name as exercise_name,
			DATE(es.created_at) as workout_date,
			MAX(es.weight) as max_weight,
			SUM(es.weight * es.reps) as total_volume
		FROM exercise_sets es
		JOIN session_exercises se ON es.session_exercise_id = se.id
		JOIN workout_sessions ws ON se.session_id = ws.id
		JOIN exercises e ON se.exercise_id = e.id
		WHERE es.completed = 1 AND ws.user_id = ?
		GROUP BY e.name, DATE(es.created_at)
		ORDER BY workout_date DESC, exercise_name
	`

	rows, err := r.sqlite.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get progress data: %w", err)
	}
	defer rows.Close()

	var progress []map[string]interface{}
	for rows.Next() {
		var exerciseName string
		var workoutDate time.Time
		var maxWeight float64
		var totalVolume float64

		err := rows.Scan(&exerciseName, &workoutDate, &maxWeight, &totalVolume)
		if err != nil {
			return nil, fmt.Errorf("failed to scan progress data: %w", err)
		}

		progress = append(progress, map[string]interface{}{
			"exerciseName": exerciseName,
			"date":         workoutDate.Format("2006-01-02"),
			"maxWeight":    maxWeight,
			"totalVolume":  totalVolume,
		})
	}

	return progress, nil
}
