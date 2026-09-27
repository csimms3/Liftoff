package repository

import (
	"context"
	"database/sql"
	"errors"
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

// StartSession starts a workout session for userID's workout, with a session
// exercise and planned sets for each of the workout's exercises. Any session the
// user still has active is ended first: a user has at most one active session.
// It all happens in one transaction, and nothing is written unless the workout
// belongs to the user (ErrNotFound otherwise).
func (r *SessionRepository) StartSession(ctx context.Context, userID, workoutID string) (*models.WorkoutSession, error) {
	workoutRepo := NewWorkoutRepository(r.db, r.sqlite, r.useSQLite)
	workout, err := workoutRepo.GetWorkout(ctx, userID, workoutID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get workout: %w", err)
	}

	err = withTx(ctx, r.db, r.sqlite, r.useSQLite, func(tx tx) error {
		if !r.useSQLite {
			// Serialize concurrent starts by the same user (SQLite serializes writers).
			if err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext(?))`, "session:"+userID); err != nil {
				return fmt.Errorf("lock: %w", err)
			}
		}
		now := time.Now()
		// End the previous active session(s) at their last logged activity, so a
		// session left open doesn't show a days-long duration in history. SQLite
		// stores timestamps as text that may carry different UTC offsets, so it
		// orders by julianday() rather than comparing the strings.
		order := "es.updated_at"
		if r.useSQLite {
			order = "julianday(es.updated_at)"
		}
		if err := tx.Exec(ctx, `
			UPDATE workout_sessions
			SET is_active = ?, updated_at = ?,
				ended_at = COALESCE((
					SELECT es.updated_at FROM exercise_sets es
					JOIN session_exercises se ON es.session_exercise_id = se.id
					WHERE se.session_id = workout_sessions.id
					ORDER BY `+order+` DESC LIMIT 1
				), started_at)
			WHERE user_id = ? AND is_active = ?`, false, now, userID, true); err != nil {
			return fmt.Errorf("end previous session: %w", err)
		}

		sessionID := uuid.New().String()
		if err := tx.Exec(ctx, `
			INSERT INTO workout_sessions (id, user_id, workout_id, started_at, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, sessionID, userID, workoutID, now, true, now, now); err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		for pos, exercise := range workout.Exercises {
			seID := uuid.New().String()
			if err := tx.Exec(ctx, `
				INSERT INTO session_exercises (id, session_id, exercise_id, position, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?)`, seID, sessionID, exercise.ID, pos, now, now); err != nil {
				return fmt.Errorf("create session exercise: %w", err)
			}
			for i := 0; i < exercise.Sets; i++ {
				if err := tx.Exec(ctx, `
					INSERT INTO exercise_sets (id, session_exercise_id, reps, weight, completed, position, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, uuid.New().String(), seID, exercise.Reps, exercise.Weight, false, i, now, now); err != nil {
					return fmt.Errorf("create exercise set: %w", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
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

		previous, err := r.previousSets(ctx, userID, se.ExerciseID, session.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get previous sets: %w", err)
		}
		se.Previous = previous
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
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
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
		return nil, ErrNotFound
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

// CreateSessionExercise adds an exercise to the user's session. Both the session
// and the exercise must belong to userID (ErrNotFound otherwise).
func (r *SessionRepository) CreateSessionExercise(ctx context.Context, userID, sessionID, exerciseID string) (*models.SessionExercise, error) {
	session, err := r.getSessionForUser(ctx, userID, sessionID)
	if err != nil || session == nil {
		return nil, ErrNotFound
	}
	if !r.ownsExercise(ctx, userID, exerciseID) {
		return nil, ErrNotFound
	}
	if r.useSQLite {
		return r.createSessionExerciseSQLite(ctx, sessionID, exerciseID)
	}
	return r.createSessionExercisePostgres(ctx, sessionID, exerciseID)
}

// ownsExercise reports whether exerciseID belongs to one of userID's workouts.
func (r *SessionRepository) ownsExercise(ctx context.Context, userID, exerciseID string) bool {
	query := `SELECT 1 FROM exercises e JOIN workouts w ON e.workout_id = w.id WHERE e.id = ? AND w.user_id = ?`
	var one int
	if r.useSQLite {
		return r.sqlite.QueryRowContext(ctx, query, exerciseID, userID).Scan(&one) == nil
	}
	return r.db.QueryRow(ctx, rebind(query), exerciseID, userID).Scan(&one) == nil
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
		INSERT INTO session_exercises (id, session_id, exercise_id, position, created_at, updated_at)
		VALUES ($1, $2, $3, (SELECT COALESCE(MAX(position) + 1, 0) FROM session_exercises WHERE session_id = $6), $4, $5)
		RETURNING id, session_id, exercise_id, created_at, updated_at
	`

	var sessionExercise models.SessionExercise
	err := r.db.QueryRow(ctx, query, id, sessionID, exerciseID, now, now, sessionID).Scan(
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
		INSERT INTO session_exercises (id, session_id, exercise_id, position, created_at, updated_at)
		VALUES (?, ?, ?, (SELECT COALESCE(MAX(position) + 1, 0) FROM session_exercises WHERE session_id = ?), ?, ?)
	`

	_, err := r.sqlite.ExecContext(ctx, query, id, sessionID, exerciseID, sessionID, now, now)
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
		ORDER BY position, created_at, id
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
		ORDER BY position, created_at, id
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
	if !r.verifySessionExerciseAccess(ctx, userID, set.SessionExerciseID) {
		return ErrNotFound
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
		INSERT INTO exercise_sets (id, session_exercise_id, reps, weight, completed, notes, position, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, (SELECT COALESCE(MAX(position) + 1, 0) FROM exercise_sets WHERE session_exercise_id = $9), $7, $8)
	`

	_, err := r.db.Exec(ctx, query, id, set.SessionExerciseID, set.Reps, set.Weight, set.Completed, set.Notes, now, now, set.SessionExerciseID)
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
		INSERT INTO exercise_sets (id, session_exercise_id, reps, weight, completed, notes, position, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(position) + 1, 0) FROM exercise_sets WHERE session_exercise_id = ?), ?, ?)
	`

	_, err := r.sqlite.ExecContext(ctx, query, id, set.SessionExerciseID, set.Reps, set.Weight, set.Completed, set.Notes, set.SessionExerciseID, now, now)
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
		ORDER BY position, created_at, id
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
		ORDER BY position, created_at, id
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
	sessionExerciseID := set.SessionExerciseID
	if sessionExerciseID == "" {
		// Look up the set's session exercise to check ownership.
		seID, err := r.getSessionExerciseIDForSet(ctx, set.ID)
		if err != nil {
			return ErrNotFound
		}
		sessionExerciseID = seID
		set.SessionExerciseID = seID
	}
	if !r.verifySessionExerciseAccess(ctx, userID, sessionExerciseID) {
		return ErrNotFound
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
	if !r.verifySessionExerciseAccess(ctx, userID, sessionExerciseID) {
		return ErrNotFound
	}
	// Get all sets for this session exercise
	sets, err := r.GetExerciseSets(ctx, sessionExerciseID)
	if err != nil {
		return fmt.Errorf("failed to get exercise sets: %w", err)
	}

	// Check if setIndex is valid
	if setIndex < 0 || setIndex >= len(sets) {
		return fmt.Errorf("%w: %d", ErrInvalidSetIndex, setIndex)
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

// SetPatch holds the fields of an exercise set to change; nil fields are kept.
type SetPatch struct {
	Reps      *int
	Weight    *float64
	Completed *bool
}

// PatchExerciseSet updates the given fields of one of userID's sets and returns
// the updated set (ErrNotFound if it's missing or someone else's).
func (r *SessionRepository) PatchExerciseSet(ctx context.Context, userID, setID string, p SetPatch) (*models.ExerciseSet, error) {
	seID, err := r.getSessionExerciseIDForSet(ctx, setID)
	if err != nil || !r.verifySessionExerciseAccess(ctx, userID, seID) {
		return nil, ErrNotFound
	}
	var set models.ExerciseSet
	err = withTx(ctx, r.db, r.sqlite, r.useSQLite, func(tx tx) error {
		if err := tx.Exec(ctx, `
			UPDATE exercise_sets
			SET reps = COALESCE(?, reps), weight = COALESCE(?, weight),
				completed = COALESCE(?, completed), updated_at = ?
			WHERE id = ?`, p.Reps, p.Weight, p.Completed, time.Now(), setID); err != nil {
			return fmt.Errorf("update exercise set: %w", err)
		}
		return tx.QueryRow(ctx, `
			SELECT id, session_exercise_id, reps, weight, completed, notes, created_at, updated_at
			FROM exercise_sets WHERE id = ?`, []any{setID},
			&set.ID, &set.SessionExerciseID, &set.Reps, &set.Weight, &set.Completed, &set.Notes, &set.CreatedAt, &set.UpdatedAt)
	})
	if err != nil {
		return nil, err
	}
	return &set, nil
}

// DeleteExerciseSet deletes one of userID's sets (ErrNotFound if it's missing or
// someone else's).
func (r *SessionRepository) DeleteExerciseSet(ctx context.Context, userID, setID string) error {
	seID, err := r.getSessionExerciseIDForSet(ctx, setID)
	if err != nil || !r.verifySessionExerciseAccess(ctx, userID, seID) {
		return ErrNotFound
	}
	return withTx(ctx, r.db, r.sqlite, r.useSQLite, func(tx tx) error {
		return tx.Exec(ctx, `DELETE FROM exercise_sets WHERE id = ?`, setID)
	})
}

// previousSets returns the completed sets for exerciseID from userID's most recent
// other session that logged any, in order: "what you did last time".
func (r *SessionRepository) previousSets(ctx context.Context, userID, exerciseID, currentSessionID string) ([]*models.ExerciseSet, error) {
	started := "ws.started_at"
	if r.useSQLite {
		started = "julianday(ws.started_at)"
	}
	query := `
		SELECT es.id, es.session_exercise_id, es.reps, es.weight, es.completed, es.notes, es.created_at, es.updated_at
		FROM exercise_sets es
		WHERE es.completed = ? AND es.session_exercise_id = (
			SELECT se.id FROM session_exercises se
			JOIN workout_sessions ws ON ws.id = se.session_id
			WHERE se.exercise_id = ? AND ws.user_id = ? AND ws.id <> ?
				AND EXISTS (SELECT 1 FROM exercise_sets c WHERE c.session_exercise_id = se.id AND c.completed = ?)
			ORDER BY ` + started + ` DESC, se.id
			LIMIT 1
		)
		ORDER BY es.position, es.created_at, es.id`
	args := []any{true, exerciseID, userID, currentSessionID, true}

	var sets []*models.ExerciseSet
	scan := func(scan func(dest ...any) error) error {
		var set models.ExerciseSet
		if err := scan(&set.ID, &set.SessionExerciseID, &set.Reps, &set.Weight, &set.Completed, &set.Notes, &set.CreatedAt, &set.UpdatedAt); err != nil {
			return err
		}
		sets = append(sets, &set)
		return nil
	}
	if r.useSQLite {
		rows, err := r.sqlite.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			if err := scan(rows.Scan); err != nil {
				return nil, err
			}
		}
		return sets, rows.Err()
	}
	rows, err := r.db.Query(ctx, rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows.Scan); err != nil {
			return nil, err
		}
	}
	return sets, rows.Err()
}
