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

// ErrSessionHasLoggedSets is returned when discarding a session that has logged sets.
var ErrSessionHasLoggedSets = errors.New("session has logged sets")

// SetCountChange is an exercise whose number of sets differs from the workout's plan.
type SetCountChange struct {
	Name string `json:"name"`
	From int    `json:"from"`
	To   int    `json:"to"`
}

// WorkoutChanges is how a session differs from the workout it was started from.
type WorkoutChanges struct {
	Added      []string         `json:"added"`
	Removed    []string         `json:"removed"`
	SetCounts  []SetCountChange `json:"set_counts"`
	Reordered  bool             `json:"reordered"`
	HasChanges bool             `json:"has_changes"`
}

// SessionSummary is what the Finish step shows.
type SessionSummary struct {
	SessionID       string         `json:"session_id"`
	WorkoutID       string         `json:"workout_id"`
	WorkoutName     string         `json:"workout_name"`
	DurationSeconds int            `json:"duration_seconds"`
	SetsDone        int            `json:"sets_done"`
	SetsTotal       int            `json:"sets_total"`
	Volume          float64        `json:"volume"` // weight x reps of completed sets, in lbs
	Changes         WorkoutChanges `json:"changes"`
	// CanUpdateWorkout is false when the workout no longer exists.
	CanUpdateWorkout bool `json:"can_update_workout"`
}

// sessionRow is a session exercise as the diff sees it.
type sessionRow struct {
	id, exerciseID, movementID, name string
	sets                             int
	lastReps                         int // of the last logged set (else the last set)
	lastWeight                       float64
}

type planRow struct {
	id, name string
	sets     int
}

// loadDiffInputs reads the active session's exercises (in order) and its workout's.
func loadDiffInputs(ctx context.Context, tx pgx.Tx, userID, sessionID string) (workoutID string, sess []sessionRow, plan []planRow, err error) {
	var wid *string
	err = tx.QueryRow(ctx, `SELECT workout_id FROM workout_sessions WHERE id = $1 AND user_id = $2 AND is_active`, sessionID, userID).Scan(&wid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, nil, ErrNotFound
	}
	if err != nil {
		return "", nil, nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT se.id, COALESCE(se.exercise_id, ''), se.movement_id, se.name,
			(SELECT COUNT(*) FROM exercise_sets s WHERE s.session_exercise_id = se.id),
			COALESCE((SELECT s.reps FROM exercise_sets s WHERE s.session_exercise_id = se.id ORDER BY s.completed DESC, s.position DESC, s.id DESC LIMIT 1), 0),
			COALESCE((SELECT s.weight FROM exercise_sets s WHERE s.session_exercise_id = se.id ORDER BY s.completed DESC, s.position DESC, s.id DESC LIMIT 1), 0)
		FROM session_exercises se WHERE se.session_id = $1 ORDER BY se.position, se.created_at, se.id`, sessionID)
	if err != nil {
		return "", nil, nil, err
	}
	for rows.Next() {
		var r sessionRow
		if err := rows.Scan(&r.id, &r.exerciseID, &r.movementID, &r.name, &r.sets, &r.lastReps, &r.lastWeight); err != nil {
			rows.Close()
			return "", nil, nil, err
		}
		sess = append(sess, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", nil, nil, err
	}
	if wid == nil {
		return "", sess, nil, nil
	}
	workoutID = *wid
	prows, err := tx.Query(ctx, `SELECT id, name, sets FROM exercises WHERE workout_id = $1 ORDER BY position, created_at, id`, workoutID)
	if err != nil {
		return "", nil, nil, err
	}
	defer prows.Close()
	for prows.Next() {
		var p planRow
		if err := prows.Scan(&p.id, &p.name, &p.sets); err != nil {
			return "", nil, nil, err
		}
		plan = append(plan, p)
	}
	return workoutID, sess, plan, prows.Err()
}

// diffSession compares the session's exercises with the workout's plan.
func diffSession(sess []sessionRow, plan []planRow) WorkoutChanges {
	c := WorkoutChanges{Added: []string{}, Removed: []string{}, SetCounts: []SetCountChange{}}
	planByID := map[string]planRow{}
	for _, p := range plan {
		planByID[p.id] = p
	}
	inSession := map[string]bool{}
	var keptOrder []string // plan exercises still in the session, in session order
	for _, r := range sess {
		p, ok := planByID[r.exerciseID]
		if !ok {
			c.Added = append(c.Added, r.name)
			continue
		}
		inSession[p.id] = true
		keptOrder = append(keptOrder, p.id)
		if r.sets != 0 && r.sets != p.sets { // all sets deleted: leave the plan alone
			c.SetCounts = append(c.SetCounts, SetCountChange{Name: p.name, From: p.sets, To: r.sets})
		}
	}
	var planOrder []string // the same exercises in the workout's order
	for _, p := range plan {
		if !inSession[p.id] {
			c.Removed = append(c.Removed, p.name)
			continue
		}
		planOrder = append(planOrder, p.id)
	}
	for i := range keptOrder {
		if keptOrder[i] != planOrder[i] {
			c.Reordered = true
			break
		}
	}
	c.HasChanges = len(c.Added)+len(c.Removed)+len(c.SetCounts) > 0 || c.Reordered
	return c
}

// SessionSummary describes an active session for the Finish step.
func (r *SessionRepository) SessionSummary(ctx context.Context, userID, sessionID string) (*SessionSummary, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	workoutID, sess, plan, err := loadDiffInputs(ctx, tx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	s := SessionSummary{SessionID: sessionID, WorkoutID: workoutID, CanUpdateWorkout: workoutID != "", Changes: diffSession(sess, plan)}
	var started time.Time
	if err := tx.QueryRow(ctx, `SELECT workout_name, started_at FROM workout_sessions WHERE id = $1`, sessionID).Scan(&s.WorkoutName, &started); err != nil {
		return nil, err
	}
	// started_at is a TIMESTAMP (no zone) holding the server's local wall clock,
	// which pgx reads back labelled UTC; reinterpret it as local time.
	started = time.Date(started.Year(), started.Month(), started.Day(), started.Hour(), started.Minute(), started.Second(), started.Nanosecond(), time.Local)
	s.DurationSeconds = max(0, int(time.Since(started).Seconds()))
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE es.completed), COALESCE(SUM(es.weight * es.reps) FILTER (WHERE es.completed), 0)
		FROM exercise_sets es JOIN session_exercises se ON se.id = es.session_exercise_id
		WHERE se.session_id = $1`, sessionID).Scan(&s.SetsTotal, &s.SetsDone, &s.Volume); err != nil {
		return nil, err
	}
	return &s, nil
}

// FinishSession ends userID's active session. With updateWorkout, the workout is
// changed to match the session's structure (exercises added, removed, reordered
// and set counts) in the same transaction; planned weights and reps of existing
// exercises are left alone.
func (r *SessionRepository) FinishSession(ctx context.Context, userID, sessionID string, updateWorkout bool) (*models.WorkoutSession, error) {
	var session *models.WorkoutSession
	err := r.inTx(ctx, userID, func(tx pgx.Tx) error {
		if updateWorkout {
			workoutID, sess, plan, err := loadDiffInputs(ctx, tx, userID, sessionID)
			if err != nil {
				return err
			}
			if workoutID == "" {
				return ErrNotFound // the workout is gone; nothing to update
			}
			// Same lock as CreateExercise, so positions can't collide with a concurrent add.
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "workout:"+workoutID); err != nil {
				return fmt.Errorf("lock: %w", err)
			}
			if err := applyToWorkout(ctx, tx, workoutID, sess, plan); err != nil {
				return err
			}
		}
		s, err := scanSession(tx.QueryRow(ctx, `
			UPDATE workout_sessions SET ended_at = $2, is_active = false, updated_at = $2
			WHERE id = $1 AND user_id = $3 AND is_active
			RETURNING `+sessionColumns, sessionID, time.Now(), userID))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		session = s
		return err
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

func applyToWorkout(ctx context.Context, tx pgx.Tx, workoutID string, sess []sessionRow, plan []planRow) error {
	inSession := map[string]bool{}
	planByID := map[string]planRow{}
	for _, p := range plan {
		planByID[p.id] = p
	}
	for _, r := range sess {
		if _, ok := planByID[r.exerciseID]; ok {
			inSession[r.exerciseID] = true
		}
	}
	// Removed exercises (their session history stays: the link is cleared).
	for _, p := range plan {
		if !inSession[p.id] {
			if _, err := tx.Exec(ctx, `DELETE FROM exercises WHERE id = $1`, p.id); err != nil {
				return fmt.Errorf("remove exercise: %w", err)
			}
		}
	}
	now := time.Now()
	for pos, r := range sess {
		exID := r.exerciseID
		if _, ok := planByID[exID]; ok {
			// Kept: new set count and position.
			sets := r.sets
			if sets == 0 {
				sets = planByID[exID].sets // all sets deleted: keep the plan's count
			}
			if _, err := tx.Exec(ctx, `UPDATE exercises SET sets = $2, position = $3, updated_at = $4 WHERE id = $1`, exID, sets, pos, now); err != nil {
				return fmt.Errorf("update exercise: %w", err)
			}
			continue
		}
		// Added (or replaced) during the session: becomes a workout exercise.
		exID = uuid.New().String()
		reps, weight := r.lastReps, r.lastWeight
		if reps == 0 {
			reps = defaultReps
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO exercises (id, name, sets, reps, weight, workout_id, movement_id, position, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`,
			exID, r.name, max(r.sets, 1), reps, weight, workoutID, r.movementID, pos, now); err != nil {
			return fmt.Errorf("add exercise: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE session_exercises SET exercise_id = $2 WHERE id = $1`, r.id, exID); err != nil {
			return err
		}
	}
	return nil
}

// DiscardSession deletes an active session that has no logged sets.
func (r *SessionRepository) DiscardSession(ctx context.Context, userID, sessionID string) error {
	return r.inTx(ctx, userID, func(tx pgx.Tx) error {
		var logged int
		err := tx.QueryRow(ctx, `
			SELECT (SELECT COUNT(*) FROM exercise_sets es JOIN session_exercises se ON se.id = es.session_exercise_id
				WHERE se.session_id = ws.id AND es.completed)
			FROM workout_sessions ws WHERE ws.id = $1 AND ws.user_id = $2 AND ws.is_active`, sessionID, userID).Scan(&logged)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if logged > 0 {
			return ErrSessionHasLoggedSets
		}
		_, err = tx.Exec(ctx, `DELETE FROM workout_sessions WHERE id = $1`, sessionID)
		return err
	})
}
