package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"liftoff/backend/models"
	"liftoff/backend/repository"
)

func TestStartSession_CreatesExercisesAndSets(t *testing.T) {
	forEachBackend(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Push", 3, 2)

		s, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !s.IsActive || len(s.Exercises) != 2 {
			t.Fatalf("active=%v exercises=%d, want active with 2", s.IsActive, len(s.Exercises))
		}
		if n := len(s.Exercises[0].Sets) + len(s.Exercises[1].Sets); n != 5 {
			t.Errorf("got %d sets, want 5 (3+2)", n)
		}
	})
}

func TestStartSession_OtherUsersWorkoutWritesNothing(t *testing.T) {
	forEachBackend(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		owner := e.user(t, "owner@example.com")
		other := e.user(t, "other@example.com")
		w := e.workout(t, owner, "Push", 3)

		for _, id := range []string{w.ID, "no-such-workout"} {
			if _, err := e.sessions.StartSession(ctx, other, id); !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("StartSession(%q) err = %v, want ErrNotFound", id, err)
			}
		}
		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions`); n != 0 {
			t.Errorf("%d sessions written for a workout the user doesn't own", n)
		}
	})
}

// Starting a workout ends the one still active, at its last logged activity.
func TestStartSession_EndsPreviousActiveSession(t *testing.T) {
	forEachBackend(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Push", 2)

		first, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		// A set logged after the session started, in the server's zone as the app
		// writes it. On SQLite it's written at UTC-12 instead: a later instant whose
		// text sorts before the other rows', so comparing strings would pick the
		// wrong row.
		lastSet := time.Now().Add(30 * time.Minute).Truncate(time.Second)
		written := lastSet
		if e.isSQLite {
			written = lastSet.In(time.FixedZone("UTC-12", -12*60*60))
		}
		e.exec(t, `UPDATE exercise_sets SET updated_at = ? WHERE id = ?`, written, first.Exercises[0].Sets[1].ID)

		second, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		if second.ID == first.ID {
			t.Fatal("second start returned the first session")
		}
		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions WHERE user_id = ? AND is_active = ?`, me, true); n != 1 {
			t.Errorf("%d active sessions, want 1", n)
		}
		completed, err := e.sessions.GetCompletedSessions(ctx, me)
		if err != nil || len(completed) != 1 || completed[0].ID != first.ID {
			t.Fatalf("completed = %v (err %v), want just the first session", completed, err)
		}
		// Postgres TIMESTAMP columns hold wall-clock time (read back labelled UTC);
		// SQLite keeps the offset. Compare the wall clock in the server's zone.
		const wall = "2006-01-02 15:04:05"
		ended := completed[0].EndedAt
		if ended == nil {
			t.Fatal("first session has no ended_at")
		}
		got := *ended
		if e.isSQLite {
			got = got.In(time.Local)
		}
		if got.Format(wall) != lastSet.Format(wall) {
			t.Errorf("first session ended_at = %s, want its last activity %s", got.Format(wall), lastSet.Format(wall))
		}
	})
}

// A failure partway through leaves no partial session and doesn't end the
// previous one.
func TestStartSession_FailureRollsBackEverything(t *testing.T) {
	forEachBackend(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Push", 2)
		first, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}

		// Make every set insert fail, after the session and first exercise rows.
		if e.isSQLite {
			e.exec(t, `CREATE TRIGGER fail_sets BEFORE INSERT ON exercise_sets BEGIN SELECT RAISE(ABORT, 'boom'); END`)
		} else {
			e.exec(t, `ALTER TABLE exercise_sets ADD CONSTRAINT fail_sets CHECK (reps < 0) NOT VALID`)
		}
		if _, err := e.sessions.StartSession(ctx, me, w.ID); err == nil {
			t.Fatal("want an error when set inserts fail")
		}

		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions`); n != 1 {
			t.Errorf("%d sessions, want 1 (the failed start must leave nothing)", n)
		}
		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions WHERE id = ? AND is_active = ?`, first.ID, true); n != 1 {
			t.Error("the previous session was ended although the new one failed")
		}
	})
}

func TestStartSession_ConcurrentStartsLeaveOneActive(t *testing.T) {
	forEachBackend(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Push", 2)

		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := e.sessions.StartSession(ctx, me, w.ID); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions WHERE user_id = ? AND is_active = ?`, me, true); n != 1 {
			t.Errorf("%d active sessions after concurrent starts, want 1", n)
		}
	})
}

// Every set/session-exercise method checks ownership; an empty user ID is not a
// bypass.
func TestSessionMethods_RejectOtherUsers(t *testing.T) {
	forEachBackend(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		owner := e.user(t, "owner@example.com")
		other := e.user(t, "other@example.com")
		w := e.workout(t, owner, "Push", 2)
		s, err := e.sessions.StartSession(ctx, owner, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		se := s.Exercises[0]
		otherWorkout := e.workout(t, other, "Theirs", 1)

		for _, uid := range []string{other, ""} {
			if err := e.sessions.CreateExerciseSet(ctx, uid, &models.ExerciseSet{SessionExerciseID: se.ID, Reps: 5, Weight: 10}); !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("CreateExerciseSet as %q: %v, want ErrNotFound", uid, err)
			}
			set := &models.ExerciseSet{ID: se.Sets[0].ID, Reps: 1, Weight: 1, Completed: true}
			if err := e.sessions.UpdateExerciseSet(ctx, uid, set); !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("UpdateExerciseSet as %q: %v, want ErrNotFound", uid, err)
			}
			if err := e.sessions.CompleteExerciseSet(ctx, uid, se.ID, 0); !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("CompleteExerciseSet as %q: %v, want ErrNotFound", uid, err)
			}
			if _, err := e.sessions.CreateSessionExercise(ctx, uid, s.ID, w.Exercises[0].ID); !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("CreateSessionExercise as %q: %v, want ErrNotFound", uid, err)
			}
		}
		// The owner can't attach someone else's exercise to their session either.
		if _, err := e.sessions.CreateSessionExercise(ctx, owner, s.ID, otherWorkout.Exercises[0].ID); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("attaching another user's exercise: %v, want ErrNotFound", err)
		}
		// Sanity: the owner still can.
		if err := e.sessions.CompleteExerciseSet(ctx, owner, se.ID, 0); err != nil {
			t.Errorf("owner CompleteExerciseSet: %v", err)
		}
	})
}

func TestEndSession_And_CompleteSet_Errors(t *testing.T) {
	forEachBackend(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		owner := e.user(t, "owner@example.com")
		other := e.user(t, "other@example.com")
		w := e.workout(t, owner, "Push", 2)
		s, err := e.sessions.StartSession(ctx, owner, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{s.ID, "no-such-session"} {
			if _, err := e.sessions.EndSession(ctx, other, id); !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("EndSession(%q) as other user: %v, want ErrNotFound", id, err)
			}
		}
		if err := e.sessions.CompleteExerciseSet(ctx, owner, s.Exercises[0].ID, 5); !errors.Is(err, repository.ErrInvalidSetIndex) {
			t.Errorf("out-of-range set index: %v, want ErrInvalidSetIndex", err)
		}
		if _, err := e.sessions.EndSession(ctx, owner, s.ID); err != nil {
			t.Errorf("owner EndSession: %v", err)
		}
	})
}
