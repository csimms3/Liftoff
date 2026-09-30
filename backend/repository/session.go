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

type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

// StartSession starts a workout session for userID's workout, with a session
// exercise and planned sets for each of the workout's exercises. Any session the
// user still has active is ended first: a user has at most one active session.
// It all happens in one transaction, and nothing is written unless the workout
// belongs to the user (ErrNotFound otherwise).
func (r *SessionRepository) StartSession(ctx context.Context, userID, workoutID string) (*models.WorkoutSession, error) {
	workoutRepo := NewWorkoutRepository(r.db)
	workout, err := workoutRepo.GetWorkout(ctx, userID, workoutID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get workout: %w", err)
	}

	err = withTx(ctx, r.db, func(tx tx) error {
		// Serialize concurrent starts by the same user.
		if err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "session:"+userID); err != nil {
			return fmt.Errorf("lock: %w", err)
		}
		now := time.Now()
		// End the previous active session(s) at their last logged activity, so a
		// session left open doesn't show a days-long duration in history.
		if err := tx.Exec(ctx, `
			UPDATE workout_sessions
			SET is_active = $1, updated_at = $2,
				ended_at = COALESCE((
					SELECT es.updated_at FROM exercise_sets es
					JOIN session_exercises se ON es.session_exercise_id = se.id
					WHERE se.session_id = workout_sessions.id
					ORDER BY es.updated_at DESC LIMIT 1
				), started_at)
			WHERE user_id = $3 AND is_active = $4`, false, now, userID, true); err != nil {
			return fmt.Errorf("end previous session: %w", err)
		}

		sessionID := uuid.New().String()
		if err := tx.Exec(ctx, `
			INSERT INTO workout_sessions (id, user_id, workout_id, workout_name, started_at, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, sessionID, userID, workoutID, workout.Name, now, true, now, now); err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		for pos, exercise := range workout.Exercises {
			seID := uuid.New().String()
			if err := tx.Exec(ctx, `
				INSERT INTO session_exercises (id, session_id, exercise_id, movement_id, name, position, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, seID, sessionID, exercise.ID, exercise.MovementID, exercise.Name, pos, now, now); err != nil {
				return fmt.Errorf("create session exercise: %w", err)
			}
			for i := 0; i < exercise.Sets; i++ {
				if err := tx.Exec(ctx, `
					INSERT INTO exercise_sets (id, session_exercise_id, reps, weight, completed, position, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, uuid.New().String(), seID, exercise.Reps, exercise.Weight, false, i, now, now); err != nil {
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

	// Populate exercises with sets and exercise details. A movement can appear
	// more than once (e.g. top sets and back-off sets); its n-th occurrence gets
	// the n-th occurrence's sets from last time.
	occurrence := map[string]int{}
	for _, se := range sessionExercises {
		if err := r.populateSessionExercise(ctx, userID, session.ID, se, occurrence[se.MovementID]); err != nil {
			return nil, err
		}
		occurrence[se.MovementID]++
	}

	// The workout, or just its name if it has since been deleted.
	session.Workout = workoutStub(session)
	if session.WorkoutID != "" {
		workout, err := NewWorkoutRepository(r.db).GetWorkout(ctx, userID, session.WorkoutID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("failed to get workout: %w", err)
		}
		if workout != nil {
			session.Workout = workout
		}
	}
	session.Exercises = sessionExercises
	return session, nil
}

// sessionColumns and scanSession read a workout_sessions row. workout_id is NULL
// once the workout has been deleted; the session keeps workout_name.
const sessionColumns = `id, user_id, COALESCE(workout_id, ''), workout_name, started_at, ended_at, is_active, created_at, updated_at`

// workoutStub is the session's workout as far as the session knows it: its id
// (empty if deleted) and the name it was performed under.
func workoutStub(s *models.WorkoutSession) *models.Workout {
	return &models.Workout{ID: s.WorkoutID, Name: s.WorkoutName, Exercises: []models.Exercise{}}
}

func scanSession(row pgx.Row) (*models.WorkoutSession, error) {
	var s models.WorkoutSession
	err := row.Scan(&s.ID, &s.UserID, &s.WorkoutID, &s.WorkoutName, &s.StartedAt, &s.EndedAt, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetCompletedSessions returns all completed workout sessions for the user
func (r *SessionRepository) GetCompletedSessions(ctx context.Context, userID string) ([]*models.WorkoutSession, error) {
	query := `SELECT ` + sessionColumns + `
		FROM workout_sessions
		WHERE user_id = $1 AND is_active = false AND ended_at IS NOT NULL
		ORDER BY ended_at DESC`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get completed sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.WorkoutSession
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		session.Workout = workoutStub(session)
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (r *SessionRepository) GetActiveSession(ctx context.Context, userID string) (*models.WorkoutSession, error) {
	session, err := scanSession(r.db.QueryRow(ctx, `SELECT `+sessionColumns+`
		FROM workout_sessions
		WHERE user_id = $1 AND is_active = true
		ORDER BY started_at DESC
		LIMIT 1`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // no active session
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}
	return session, nil
}

func (r *SessionRepository) EndSession(ctx context.Context, userID, id string) (*models.WorkoutSession, error) {
	now := time.Now()
	var session *models.WorkoutSession
	// Under the same lock as the exercise edits, so none can slip into a session
	// that has just ended.
	err := r.inTx(ctx, userID, func(tx pgx.Tx) error {
		s, err := scanSession(tx.QueryRow(ctx, `
			UPDATE workout_sessions
			SET ended_at = $2, is_active = false, updated_at = $2
			WHERE id = $1 AND user_id = $3
			RETURNING `+sessionColumns, id, now, userID))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		session = s
		return err
	})
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to end session: %w", err)
	}
	return session, nil
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
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO session_exercises (id, session_id, exercise_id, movement_id, name, position, created_at, updated_at)
		SELECT $1, $2, e.id, e.movement_id, e.name,
			(SELECT COALESCE(MAX(position) + 1, 0) FROM session_exercises WHERE session_id = $5), $4, $4
		FROM exercises e WHERE e.id = $3
		RETURNING id, session_id, COALESCE(exercise_id, ''), movement_id, name, created_at, updated_at
	`

	var sessionExercise models.SessionExercise
	err = r.db.QueryRow(ctx, query, id, sessionID, exerciseID, now, sessionID).Scan(
		&sessionExercise.ID, &sessionExercise.SessionID, &sessionExercise.ExerciseID,
		&sessionExercise.MovementID, &sessionExercise.Name, &sessionExercise.CreatedAt, &sessionExercise.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session exercise: %w", err)
	}

	return &sessionExercise, nil
}

// ownsExercise reports whether exerciseID belongs to one of userID's workouts.
func (r *SessionRepository) ownsExercise(ctx context.Context, userID, exerciseID string) bool {
	query := `SELECT 1 FROM exercises e JOIN workouts w ON e.workout_id = w.id WHERE e.id = $1 AND w.user_id = $2`
	var one int
	return r.db.QueryRow(ctx, query, exerciseID, userID).Scan(&one) == nil
}

func (r *SessionRepository) getSessionForUser(ctx context.Context, userID, sessionID string) (*models.WorkoutSession, error) {
	query := `SELECT id FROM workout_sessions WHERE id = $1 AND user_id = $2`
	var id string
	err := r.db.QueryRow(ctx, query, sessionID, userID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &models.WorkoutSession{ID: id}, nil
}

func (r *SessionRepository) GetSessionExercises(ctx context.Context, sessionID string) ([]*models.SessionExercise, error) {
	query := `
		SELECT id, session_id, COALESCE(exercise_id, ''), movement_id, name, created_at, updated_at
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
			&sessionExercise.MovementID, &sessionExercise.Name, &sessionExercise.CreatedAt, &sessionExercise.UpdatedAt,
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

func (r *SessionRepository) getSessionExerciseIDForSet(ctx context.Context, setID string) (string, error) {
	var sessionExerciseID string
	err := r.db.QueryRow(ctx, `SELECT session_exercise_id FROM exercise_sets WHERE id = $1`, setID).Scan(&sessionExerciseID)
	return sessionExerciseID, err
}

func (r *SessionRepository) verifySessionExerciseAccess(ctx context.Context, userID, sessionExerciseID string) bool {
	var one int
	return r.db.QueryRow(ctx, `SELECT 1 FROM session_exercises se JOIN workout_sessions ws ON se.session_id = ws.id
		WHERE se.id = $1 AND ws.user_id = $2`, sessionExerciseID, userID).Scan(&one) == nil
}

func (r *SessionRepository) GetExerciseSets(ctx context.Context, sessionExerciseID string) ([]*models.ExerciseSet, error) {
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
	query := `
		SELECT 
			m.name as exercise_name,
			DATE(es.created_at) as workout_date,
			MAX(es.weight) as max_weight,
			SUM(es.weight * es.reps) as total_volume
		FROM exercise_sets es
		JOIN session_exercises se ON es.session_exercise_id = se.id
		JOIN workout_sessions ws ON se.session_id = ws.id
		JOIN movements m ON m.id = se.movement_id
		WHERE es.completed = true AND ws.user_id = $1
		GROUP BY m.id, m.name, DATE(es.created_at)
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
	err = withTx(ctx, r.db, func(tx tx) error {
		if err := tx.Exec(ctx, `
			UPDATE exercise_sets
			SET reps = COALESCE($1, reps), weight = COALESCE($2, weight),
				completed = COALESCE($3, completed), updated_at = $4
			WHERE id = $5`, p.Reps, p.Weight, p.Completed, time.Now(), setID); err != nil {
			return fmt.Errorf("update exercise set: %w", err)
		}
		return tx.QueryRow(ctx, `
			SELECT id, session_exercise_id, reps, weight, completed, notes, created_at, updated_at
			FROM exercise_sets WHERE id = $1`, []any{setID},
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
	return withTx(ctx, r.db, func(tx tx) error {
		return tx.Exec(ctx, `DELETE FROM exercise_sets WHERE id = $1`, setID)
	})
}

// previousSets returns "what you did last time" for movementID: the completed sets
// from userID's most recent other session that logged any, whichever workout that
// was in. When more than one of that session's rows for the movement logged sets,
// occurrence (0-based, in position order) picks which; past the last, the last.
func (r *SessionRepository) previousSets(ctx context.Context, userID, movementID, currentSessionID string, occurrence int) ([]*models.ExerciseSet, error) {
	query := `
		WITH last AS (
			SELECT ws.id FROM workout_sessions ws
			JOIN session_exercises se ON se.session_id = ws.id
			WHERE se.movement_id = $1 AND ws.user_id = $2 AND ws.id <> $3
				AND EXISTS (SELECT 1 FROM exercise_sets c WHERE c.session_exercise_id = se.id AND c.completed)
			ORDER BY ws.started_at DESC, ws.id
			LIMIT 1
		), occurrences AS (
			-- Only rows that logged something: a quick log leaves the planned row
			-- empty and puts the set in an extra row after it.
			SELECT se.id, ROW_NUMBER() OVER (ORDER BY se.position, se.created_at, se.id) - 1 AS n
			FROM session_exercises se
			WHERE se.session_id = (SELECT id FROM last) AND se.movement_id = $1
				AND EXISTS (SELECT 1 FROM exercise_sets c WHERE c.session_exercise_id = se.id AND c.completed)
		)
		SELECT es.id, es.session_exercise_id, es.reps, es.weight, es.completed, es.notes, es.created_at, es.updated_at
		FROM exercise_sets es
		WHERE es.completed AND es.session_exercise_id = (
			SELECT id FROM occurrences WHERE n = LEAST($4, (SELECT MAX(n) FROM occurrences))
		)
		ORDER BY es.position, es.created_at, es.id`
	rows, err := r.db.Query(ctx, query, movementID, userID, currentSessionID, occurrence)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sets []*models.ExerciseSet
	for rows.Next() {
		var set models.ExerciseSet
		if err := rows.Scan(&set.ID, &set.SessionExerciseID, &set.Reps, &set.Weight, &set.Completed, &set.Notes, &set.CreatedAt, &set.UpdatedAt); err != nil {
			return nil, err
		}
		sets = append(sets, &set)
	}
	return sets, rows.Err()
}

// populateSessionExercise fills in a session exercise's plan, sets and previous
// sets. occurrence is its 0-based index among the session's rows for the same movement.
func (r *SessionRepository) populateSessionExercise(ctx context.Context, userID, sessionID string, se *models.SessionExercise, occurrence int) error {
	// The workout's exercise (its plan), or just the name if it has since been
	// removed from the workout or replaced.
	se.Exercise = &models.Exercise{Name: se.Name, MovementID: se.MovementID}
	if se.ExerciseID != "" {
		exercise, err := NewWorkoutRepository(r.db).GetExercise(ctx, se.ExerciseID)
		if err != nil {
			return fmt.Errorf("failed to get exercise: %w", err)
		}
		se.Exercise = exercise
	}
	sets, err := r.GetExerciseSets(ctx, se.ID)
	if err != nil {
		return fmt.Errorf("failed to get exercise sets: %w", err)
	}
	se.Sets = sets
	previous, err := r.previousSets(ctx, userID, se.MovementID, sessionID, occurrence)
	if err != nil {
		return fmt.Errorf("failed to get previous sets: %w", err)
	}
	se.Previous = previous
	return nil
}
