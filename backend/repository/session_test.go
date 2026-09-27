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
	withDB(t, func(t *testing.T, e *env) {
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
	withDB(t, func(t *testing.T, e *env) {
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
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Push", 2)

		first, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		// A set logged after the session started, written as the app writes times.
		lastSet := time.Now().Add(30 * time.Minute).Truncate(time.Second)
		e.exec(t, `UPDATE exercise_sets SET updated_at = $1 WHERE id = $2`, lastSet, first.Exercises[0].Sets[1].ID)

		second, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		if second.ID == first.ID {
			t.Fatal("second start returned the first session")
		}
		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions WHERE user_id = $1 AND is_active = $2`, me, true); n != 1 {
			t.Errorf("%d active sessions, want 1", n)
		}
		completed, err := e.sessions.GetCompletedSessions(ctx, me)
		if err != nil || len(completed) != 1 || completed[0].ID != first.ID {
			t.Fatalf("completed = %v (err %v), want just the first session", completed, err)
		}
		// TIMESTAMP columns hold wall-clock time (read back labelled UTC), so compare
		// the wall clock in the server's zone.
		const wall = "2006-01-02 15:04:05"
		ended := completed[0].EndedAt
		if ended == nil {
			t.Fatal("first session has no ended_at")
		}
		got := *ended
		if got.Format(wall) != lastSet.Format(wall) {
			t.Errorf("first session ended_at = %s, want its last activity %s", got.Format(wall), lastSet.Format(wall))
		}
	})
}

// A failure partway through leaves no partial session and doesn't end the
// previous one.
func TestStartSession_FailureRollsBackEverything(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Push", 2)
		first, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}

		// Make every set insert fail, after the session and first exercise rows.
		e.exec(t, `ALTER TABLE exercise_sets ADD CONSTRAINT fail_sets CHECK (reps < 0) NOT VALID`)
		if _, err := e.sessions.StartSession(ctx, me, w.ID); err == nil {
			t.Fatal("want an error when set inserts fail")
		}

		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions`); n != 1 {
			t.Errorf("%d sessions, want 1 (the failed start must leave nothing)", n)
		}
		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions WHERE id = $1 AND is_active = $2`, first.ID, true); n != 1 {
			t.Error("the previous session was ended although the new one failed")
		}
	})
}

func TestStartSession_ConcurrentStartsLeaveOneActive(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
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
		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions WHERE user_id = $1 AND is_active = $2`, me, true); n != 1 {
			t.Errorf("%d active sessions after concurrent starts, want 1", n)
		}
	})
}

// Every set/session-exercise method checks ownership; an empty user ID is not a
// bypass.
func TestSessionMethods_RejectOtherUsers(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
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
	withDB(t, func(t *testing.T, e *env) {
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

func TestPatchExerciseSet(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		other := e.user(t, "other@example.com")
		w := e.workout(t, me, "Push", 2)
		s, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		setID := s.Exercises[0].Sets[0].ID
		reps, weight, done, undone := 5, 102.5, true, false

		got, err := e.sessions.PatchExerciseSet(ctx, me, setID, repository.SetPatch{Reps: &reps})
		if err != nil || got.Reps != 5 || got.Weight != 50 || got.Completed {
			t.Fatalf("reps only: %+v %v; want reps 5, weight unchanged 50, not completed", got, err)
		}
		got, _ = e.sessions.PatchExerciseSet(ctx, me, setID, repository.SetPatch{Weight: &weight, Completed: &done})
		if got.Reps != 5 || got.Weight != 102.5 || !got.Completed {
			t.Errorf("weight+completed: %+v", got)
		}
		got, _ = e.sessions.PatchExerciseSet(ctx, me, setID, repository.SetPatch{Completed: &undone})
		if got.Completed || got.Weight != 102.5 {
			t.Errorf("untick: %+v, want not completed and weight kept", got)
		}
		for _, uid := range []string{other, ""} {
			if _, err := e.sessions.PatchExerciseSet(ctx, uid, setID, repository.SetPatch{Reps: &reps}); !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("patch as %q: %v, want ErrNotFound", uid, err)
			}
		}
	})
}

func TestDeleteExerciseSet(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		other := e.user(t, "other@example.com")
		w := e.workout(t, me, "Push", 3)
		s, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		sets := s.Exercises[0].Sets
		if err := e.sessions.DeleteExerciseSet(ctx, other, sets[2].ID); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("delete as other user: %v, want ErrNotFound", err)
		}
		if err := e.sessions.DeleteExerciseSet(ctx, me, sets[2].ID); err != nil {
			t.Fatal(err)
		}
		left, _ := e.sessions.GetExerciseSets(ctx, s.Exercises[0].ID)
		if len(left) != 2 || left[0].ID != sets[0].ID || left[1].ID != sets[1].ID {
			t.Errorf("after deleting set 3: %d sets left, want sets 1 and 2 in order", len(left))
		}
	})
}

// Sets keep their order after edits (on Postgres an UPDATE can move a row, and
// all planned sets share one created_at), and new sets go last.
func TestExerciseSets_StableOrder(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Push", 4, 3, 2)
		s, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		for i, se := range s.Exercises {
			if se.ExerciseID != w.Exercises[i].ID {
				t.Fatalf("session exercise %d is %s, want the workout's order", i, se.ExerciseID)
			}
		}
		se := s.Exercises[0]
		want := []string{}
		for _, set := range se.Sets {
			want = append(want, set.ID)
		}
		for i := len(se.Sets) - 1; i >= 0; i-- { // edit in reverse to shuffle physical order
			r := 10 + i
			if _, err := e.sessions.PatchExerciseSet(ctx, me, se.Sets[i].ID, repository.SetPatch{Reps: &r}); err != nil {
				t.Fatal(err)
			}
		}
		added := &models.ExerciseSet{SessionExerciseID: se.ID, Reps: 1, Weight: 1}
		if err := e.sessions.CreateExerciseSet(ctx, me, added); err != nil {
			t.Fatal(err)
		}
		want = append(want, added.ID)
		got, _ := e.sessions.GetExerciseSets(ctx, se.ID)
		for i := range want {
			if i >= len(got) || got[i].ID != want[i] {
				t.Fatalf("set order changed: position %d", i)
			}
		}
	})
}

func TestActiveSession_PreviousSets(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Push", 3)
		first, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		if p := first.Exercises[0].Previous; len(p) != 0 {
			t.Fatalf("first session has %d previous sets, want none", len(p))
		}
		// Log sets 1 and 2 (with different values); leave set 3 unlogged.
		done := true
		for i, reps := range []int{8, 6} {
			wt := 100.0 + float64(i)*5
			if _, err := e.sessions.PatchExerciseSet(ctx, me, first.Exercises[0].Sets[i].ID, repository.SetPatch{Reps: &reps, Weight: &wt, Completed: &done}); err != nil {
				t.Fatal(err)
			}
		}
		second, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		p := second.Exercises[0].Previous
		if len(p) != 2 || p[0].Reps != 8 || p[0].Weight != 100 || p[1].Reps != 6 || p[1].Weight != 105 {
			t.Errorf("previous = %d sets %v, want [100x8, 105x6]", len(p), p)
		}
	})
}

// Exercises added to a session mid-workout go after the planned ones.
func TestCreateSessionExercise_AppendsInOrder(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Push", 2, 2)
		extra := e.workout(t, me, "Extra", 1)
		s, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.sessions.CreateSessionExercise(ctx, me, s.ID, extra.Exercises[0].ID); err != nil {
			t.Fatal(err)
		}
		got, err := e.sessions.GetActiveSessionWithExercises(ctx, me)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Exercises) != 3 || got.Exercises[2].ExerciseID != extra.Exercises[0].ID {
			t.Errorf("added exercise should be last of 3")
		}
	})
}

func TestPatchExerciseSet_DeletedSetIsNotFound(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Push", 2)
		s, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		id := s.Exercises[0].Sets[1].ID
		if err := e.sessions.DeleteExerciseSet(ctx, me, id); err != nil {
			t.Fatal(err)
		}
		reps := 3
		if _, err := e.sessions.PatchExerciseSet(ctx, me, id, repository.SetPatch{Reps: &reps}); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("patching a deleted set: %v, want ErrNotFound", err)
		}
	})
}
