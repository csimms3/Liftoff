package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"liftoff/backend/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Default plan for an exercise added to a session with no history.
const (
	defaultSets = 3
	defaultReps = 10
)

// MovementSummary is a movement with when the user last did it, for the picker.
type MovementSummary struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Category string     `json:"category"`
	LastUsed *time.Time `json:"last_used"`
}

// ListMovements returns userID's movements, most recently done first, then by name.
func (r *SessionRepository) ListMovements(ctx context.Context, userID string) ([]MovementSummary, error) {
	rows, err := r.db.Query(ctx, `
		SELECT m.id, m.name, m.category, MAX(ws.started_at)
		FROM movements m
		LEFT JOIN session_exercises se ON se.movement_id = m.id
		LEFT JOIN workout_sessions ws ON ws.id = se.session_id AND ws.user_id = m.user_id
		WHERE m.user_id = $1
		GROUP BY m.id
		ORDER BY MAX(ws.started_at) DESC NULLS LAST, LOWER(m.name)`, userID)
	if err != nil {
		return nil, fmt.Errorf("list movements: %w", err)
	}
	defer rows.Close()
	out := []MovementSummary{}
	for rows.Next() {
		var m MovementSummary
		if err := rows.Scan(&m.ID, &m.Name, &m.Category, &m.LastUsed); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// lockUserSessions serializes session edits by the same user (shared with StartSession).
func lockUserSessions(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "session:"+userID)
	return err
}

// activeSessionOf returns the id of the active session that owns the session
// exercise, or ErrNotFound (missing, someone else's, or already ended).
func activeSessionOf(ctx context.Context, tx pgx.Tx, userID, sessionExerciseID string) (string, error) {
	var sessionID string
	err := tx.QueryRow(ctx, `
		SELECT ws.id FROM session_exercises se
		JOIN workout_sessions ws ON ws.id = se.session_id
		WHERE se.id = $1 AND ws.user_id = $2 AND ws.is_active`, sessionExerciseID, userID).Scan(&sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return sessionID, err
}

// resolveMovement returns the id and name of one of userID's movements, given its
// id or, failing that, a name to find or create.
func resolveMovement(ctx context.Context, tx pgx.Tx, userID, movementID, name string) (string, string, error) {
	if movementID != "" {
		var n string
		err := tx.QueryRow(ctx, `SELECT name FROM movements WHERE id = $1 AND user_id = $2`, movementID, userID).Scan(&n)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return movementID, n, err
	}
	id, err := findOrCreateMovement(ctx, tx, userID, name)
	if err != nil {
		return "", "", err
	}
	var n string
	err = tx.QueryRow(ctx, `SELECT name FROM movements WHERE id = $1`, id).Scan(&n)
	return id, n, err
}

// renumber makes the positions of a session's exercises 0..n-1 in their current order.
func renumber(ctx context.Context, tx pgx.Tx, sessionID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE session_exercises se SET position = r.n
		FROM (SELECT id, ROW_NUMBER() OVER (ORDER BY position, created_at, id) - 1 AS n
			FROM session_exercises WHERE session_id = $1) r
		WHERE se.id = r.id AND se.position <> r.n`, sessionID)
	return err
}

// AddMovementToSession adds an exercise to userID's active session, by movement id
// or by name (found or created). Its planned sets copy the last session that
// logged the movement, or 3x10 at weight 0. position nil appends; otherwise it is
// inserted there (clamped) and later exercises shift down.
func (r *SessionRepository) AddMovementToSession(ctx context.Context, userID, sessionID, movementID, name string, position *int) (*models.SessionExercise, error) {
	var seID string
	err := r.inTx(ctx, userID, func(tx pgx.Tx) error {
		var one int
		err := tx.QueryRow(ctx, `SELECT 1 FROM workout_sessions WHERE id = $1 AND user_id = $2 AND is_active`, sessionID, userID).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		mid, mname, err := resolveMovement(ctx, tx, userID, movementID, name)
		if err != nil {
			return err
		}

		var count int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM session_exercises WHERE session_id = $1`, sessionID).Scan(&count); err != nil {
			return err
		}
		pos := count
		if position != nil {
			pos = min(max(*position, 0), count)
			if _, err := tx.Exec(ctx, `UPDATE session_exercises SET position = position + 1 WHERE session_id = $1 AND position >= $2`, sessionID, pos); err != nil {
				return err
			}
		}

		now := time.Now()
		seID = uuid.New().String()
		if _, err := tx.Exec(ctx, `
			INSERT INTO session_exercises (id, session_id, exercise_id, movement_id, name, position, created_at, updated_at)
			VALUES ($1, $2, NULL, $3, $4, $5, $6, $6)`, seID, sessionID, mid, mname, pos, now); err != nil {
			return fmt.Errorf("add session exercise: %w", err)
		}

		// Plan: last time's logged sets, else the default.
		rows, err := tx.Query(ctx, `
			SELECT es.reps, es.weight FROM exercise_sets es
			WHERE es.completed AND es.session_exercise_id = (
				SELECT se.id FROM session_exercises se
				JOIN workout_sessions ws ON ws.id = se.session_id
				WHERE se.movement_id = $1 AND ws.user_id = $2 AND ws.id <> $3
					AND EXISTS (SELECT 1 FROM exercise_sets c WHERE c.session_exercise_id = se.id AND c.completed)
				ORDER BY ws.started_at DESC, se.position, se.id LIMIT 1)
			ORDER BY es.position, es.created_at, es.id`, mid, userID, sessionID)
		if err != nil {
			return err
		}
		type plan struct {
			reps   int
			weight float64
		}
		var plans []plan
		for rows.Next() {
			var p plan
			if err := rows.Scan(&p.reps, &p.weight); err != nil {
				rows.Close()
				return err
			}
			plans = append(plans, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(plans) == 0 {
			for i := 0; i < defaultSets; i++ {
				plans = append(plans, plan{defaultReps, 0})
			}
		}
		for i, p := range plans {
			if _, err := tx.Exec(ctx, `
				INSERT INTO exercise_sets (id, session_exercise_id, reps, weight, completed, position, created_at, updated_at)
				VALUES ($1, $2, $3, $4, false, $5, $6, $6)`, uuid.New().String(), seID, p.reps, p.weight, i, now); err != nil {
				return fmt.Errorf("add planned set: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.loadSessionExercise(ctx, userID, seID)
}

// RemoveSessionExercise removes an exercise (and its sets) from an active session.
func (r *SessionRepository) RemoveSessionExercise(ctx context.Context, userID, sessionExerciseID string) error {
	return r.inTx(ctx, userID, func(tx pgx.Tx) error {
		sessionID, err := activeSessionOf(ctx, tx, userID, sessionExerciseID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM session_exercises WHERE id = $1`, sessionExerciseID); err != nil {
			return fmt.Errorf("remove session exercise: %w", err)
		}
		return renumber(ctx, tx, sessionID)
	})
}

// MoveSessionExercise moves an exercise to position (clamped) within its session.
func (r *SessionRepository) MoveSessionExercise(ctx context.Context, userID, sessionExerciseID string, position int) error {
	return r.inTx(ctx, userID, func(tx pgx.Tx) error {
		sessionID, err := activeSessionOf(ctx, tx, userID, sessionExerciseID)
		if err != nil {
			return err
		}
		if err := renumber(ctx, tx, sessionID); err != nil {
			return err
		}
		var count, current int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*), (SELECT position FROM session_exercises WHERE id = $2)
			FROM session_exercises WHERE session_id = $1`, sessionID, sessionExerciseID).Scan(&count, &current); err != nil {
			return err
		}
		target := min(max(position, 0), count-1)
		switch {
		case target < current:
			_, err = tx.Exec(ctx, `UPDATE session_exercises SET position = position + 1 WHERE session_id = $1 AND position >= $2 AND position < $3`, sessionID, target, current)
		case target > current:
			_, err = tx.Exec(ctx, `UPDATE session_exercises SET position = position - 1 WHERE session_id = $1 AND position > $2 AND position <= $3`, sessionID, current, target)
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE session_exercises SET position = $2 WHERE id = $1`, sessionExerciseID, target)
		return err
	})
}

// ReplaceSessionExercise swaps the movement of an exercise in an active session,
// keeping its sets and position. The link to the workout's plan is cleared.
func (r *SessionRepository) ReplaceSessionExercise(ctx context.Context, userID, sessionExerciseID, movementID, name string) (*models.SessionExercise, error) {
	err := r.inTx(ctx, userID, func(tx pgx.Tx) error {
		if _, err := activeSessionOf(ctx, tx, userID, sessionExerciseID); err != nil {
			return err
		}
		mid, mname, err := resolveMovement(ctx, tx, userID, movementID, name)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE session_exercises SET movement_id = $2, name = $3, exercise_id = NULL, updated_at = $4 WHERE id = $1`,
			sessionExerciseID, mid, mname, time.Now())
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.loadSessionExercise(ctx, userID, sessionExerciseID)
}

// inTx runs fn in a transaction holding the user's session lock.
func (r *SessionRepository) inTx(ctx context.Context, userID string, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockUserSessions(ctx, tx, userID); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// loadSessionExercise returns one session exercise with plan, sets and previous.
func (r *SessionRepository) loadSessionExercise(ctx context.Context, userID, id string) (*models.SessionExercise, error) {
	var se models.SessionExercise
	var occurrence int
	err := r.db.QueryRow(ctx, `
		SELECT se.id, se.session_id, COALESCE(se.exercise_id, ''), se.movement_id, se.name, se.created_at, se.updated_at,
			(SELECT COUNT(*) FROM session_exercises o WHERE o.session_id = se.session_id AND o.movement_id = se.movement_id
				AND (o.position, o.created_at, o.id) < (se.position, se.created_at, se.id))
		FROM session_exercises se
		JOIN workout_sessions ws ON ws.id = se.session_id
		WHERE se.id = $1 AND ws.user_id = $2`, id, userID).Scan(
		&se.ID, &se.SessionID, &se.ExerciseID, &se.MovementID, &se.Name, &se.CreatedAt, &se.UpdatedAt, &occurrence)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := r.populateSessionExercise(ctx, userID, se.SessionID, &se, occurrence); err != nil {
		return nil, err
	}
	return &se, nil
}
