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

func names(w *models.Workout) []string {
	var out []string
	for _, e := range w.Exercises {
		out = append(out, e.Name)
	}
	return out
}

func TestSummary_CountsAndNoChanges(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e) // Push-ex0, Push-ex1 with 2 sets each
		w, r, done := 100.0, 5, true
		if _, err := e.sessions.PatchExerciseSet(ctx, me, s.Exercises[0].Sets[0].ID, repository.SetPatch{Weight: &w, Reps: &r, Completed: &done}); err != nil {
			t.Fatal(err)
		}
		sum, err := e.sessions.SessionSummary(ctx, me, s.ID)
		if err != nil {
			t.Fatal(err)
		}
		if sum.SetsDone != 1 || sum.SetsTotal != 4 || sum.Volume != 500 || sum.WorkoutName != "Push" || !sum.CanUpdateWorkout {
			t.Errorf("summary = %+v", sum)
		}
		if sum.Changes.HasChanges {
			t.Errorf("unchanged session reports changes: %+v", sum.Changes)
		}
	})
}

func TestSummary_DetectsAddedRemovedReorderedAndSetCounts(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Legs", 2, 2, 2) // Legs-ex0..2
		s, _ := e.sessions.StartSession(ctx, me, w.ID)
		e.sessions.RemoveSessionExercise(ctx, me, s.Exercises[2].ID)                         // remove ex2
		e.sessions.AddMovementToSession(ctx, me, s.ID, "", "Lunge", nil)                     // add
		e.sessions.MoveSessionExercise(ctx, me, s.Exercises[1].ID, 0)                        // ex1 before ex0
		set := &models.ExerciseSet{SessionExerciseID: s.Exercises[0].ID, Reps: 5, Weight: 1} // ex0: 3 sets
		e.sessions.CreateExerciseSet(ctx, me, set)

		c := mustSummary(t, e, me, s.ID).Changes
		if len(c.Added) != 1 || c.Added[0] != "Lunge" || len(c.Removed) != 1 || c.Removed[0] != "Legs-ex2" {
			t.Errorf("added/removed = %v / %v", c.Added, c.Removed)
		}
		if len(c.SetCounts) != 1 || c.SetCounts[0] != (repository.SetCountChange{Name: "Legs-ex0", From: 2, To: 3}) {
			t.Errorf("set counts = %+v", c.SetCounts)
		}
		if !c.Reordered || !c.HasChanges {
			t.Errorf("reordered=%v has=%v", c.Reordered, c.HasChanges)
		}
	})
}

func mustSummary(t *testing.T, e *env, userID, sessionID string) *repository.SessionSummary {
	t.Helper()
	sum, err := e.sessions.SessionSummary(context.Background(), userID, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func TestFinish_UpdateWorkoutAppliesStructureOnly(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Legs", 2, 2, 2)
		s, _ := e.sessions.StartSession(ctx, me, w.ID)
		e.sessions.RemoveSessionExercise(ctx, me, s.Exercises[2].ID)
		added, _ := e.sessions.AddMovementToSession(ctx, me, s.ID, "", "Lunge", nil)
		e.sessions.MoveSessionExercise(ctx, me, s.Exercises[1].ID, 0)
		e.sessions.CreateExerciseSet(ctx, me, &models.ExerciseSet{SessionExerciseID: s.Exercises[0].ID, Reps: 5, Weight: 1})
		done, wt, reps := true, 90.0, 6
		e.sessions.PatchExerciseSet(ctx, me, added.Sets[0].ID, repository.SetPatch{Weight: &wt, Reps: &reps, Completed: &done})

		if _, err := e.sessions.FinishSession(ctx, me, s.ID, true); err != nil {
			t.Fatal(err)
		}
		got, err := e.workouts.GetWorkout(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"Legs-ex1", "Legs-ex0", "Lunge"}; !equal(names(got), want) {
			t.Errorf("workout exercises = %v, want %v", names(got), want)
		}
		for _, ex := range got.Exercises {
			switch ex.Name {
			case "Legs-ex0":
				if ex.Sets != 3 || ex.Reps != 8 || ex.Weight != 50 {
					t.Errorf("ex0 = %+v, want 3 sets, planned reps/weight untouched", ex)
				}
			case "Lunge":
				if ex.Sets != 3 || ex.Reps != 10 && ex.Reps != 6 {
					t.Errorf("Lunge = %+v", ex)
				}
			}
		}
		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions WHERE id = $1 AND NOT is_active AND ended_at IS NOT NULL`, s.ID); n != 1 {
			t.Error("session not ended")
		}
		// The added exercise's logged set is kept and now linked to the workout's exercise.
		if n := e.count(t, `SELECT COUNT(*) FROM exercise_sets WHERE completed`); n != 1 {
			t.Errorf("%d logged sets, want 1", n)
		}
		// Starting again uses the updated workout.
		s2, _ := e.sessions.StartSession(ctx, me, w.ID)
		if len(s2.Exercises) != 3 || s2.Exercises[0].Name != "Legs-ex1" {
			t.Errorf("next session = %d exercises, first %q", len(s2.Exercises), s2.Exercises[0].Name)
		}
	})
}

func TestFinish_WithoutUpdateLeavesWorkout(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Legs", 2, 2)
		s, _ := e.sessions.StartSession(ctx, me, w.ID)
		e.sessions.RemoveSessionExercise(ctx, me, s.Exercises[1].ID)
		if _, err := e.sessions.FinishSession(ctx, me, s.ID, false); err != nil {
			t.Fatal(err)
		}
		got, _ := e.workouts.GetWorkout(ctx, me, w.ID)
		if len(got.Exercises) != 2 {
			t.Errorf("workout has %d exercises, want unchanged 2", len(got.Exercises))
		}
	})
}

func TestFinish_OwnershipAndState(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e)
		other := e.user(t, "other@example.com")
		if _, err := e.sessions.FinishSession(ctx, other, s.ID, false); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("finish as other user: %v", err)
		}
		if _, err := e.sessions.SessionSummary(ctx, other, s.ID); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("summary as other user: %v", err)
		}
		if _, err := e.sessions.FinishSession(ctx, me, s.ID, false); err != nil {
			t.Fatal(err)
		}
		if _, err := e.sessions.FinishSession(ctx, me, s.ID, false); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("finishing an ended session: %v", err)
		}
	})
}

func TestFinish_WorkoutDeletedMidSession(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e)
		w, _ := e.workouts.GetWorkout(ctx, me, s.WorkoutID)
		e.workouts.DeleteWorkout(ctx, me, w.ID)
		sum := mustSummary(t, e, me, s.ID)
		if sum.CanUpdateWorkout || sum.WorkoutName != "Push" {
			t.Errorf("summary = %+v, want no workout update, name kept", sum)
		}
		if _, err := e.sessions.FinishSession(ctx, me, s.ID, true); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("update of a deleted workout: %v, want ErrNotFound", err)
		}
		if _, err := e.sessions.FinishSession(ctx, me, s.ID, false); err != nil {
			t.Errorf("plain finish should still work: %v", err)
		}
	})
}

func TestDiscard(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e)
		other := e.user(t, "other@example.com")
		if err := e.sessions.DiscardSession(ctx, other, s.ID); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("discard as other user: %v", err)
		}
		done := true
		e.sessions.PatchExerciseSet(ctx, me, s.Exercises[0].Sets[0].ID, repository.SetPatch{Completed: &done})
		if err := e.sessions.DiscardSession(ctx, me, s.ID); !errors.Is(err, repository.ErrSessionHasLoggedSets) {
			t.Errorf("discard with a logged set: %v, want ErrSessionHasLoggedSets", err)
		}
		undone := false
		e.sessions.PatchExerciseSet(ctx, me, s.Exercises[0].Sets[0].ID, repository.SetPatch{Completed: &undone})
		if err := e.sessions.DiscardSession(ctx, me, s.ID); err != nil {
			t.Fatal(err)
		}
		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions`) + e.count(t, `SELECT COUNT(*) FROM exercise_sets`); n != 0 {
			t.Errorf("%d rows left after discard", n)
		}
		if active, _ := e.sessions.GetActiveSessionWithExercises(ctx, me); active != nil {
			t.Error("still an active session")
		}
	})
}

func TestWorkoutExerciseOrderIsExplicit(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "W", 1, 1, 1)
		if want := []string{"W-ex0", "W-ex1", "W-ex2"}; !equal(names(w), want) {
			t.Errorf("order = %v", names(w))
		}
		if n := e.count(t, `SELECT COUNT(DISTINCT position) FROM exercises`); n != 3 {
			t.Errorf("%d distinct positions, want 3", n)
		}
	})
}

// An exercise added mid-session takes its plan from the set actually logged, not
// from an unlogged planned set.
func TestFinish_AddedExercisePlanUsesLoggedSet(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e)
		added, _ := e.sessions.AddMovementToSession(ctx, me, s.ID, "", "Curl", nil) // 3x10@0
		wt, reps, done := 30.0, 12, true
		e.sessions.PatchExerciseSet(ctx, me, added.Sets[0].ID, repository.SetPatch{Weight: &wt, Reps: &reps, Completed: &done})
		if _, err := e.sessions.FinishSession(ctx, me, s.ID, true); err != nil {
			t.Fatal(err)
		}
		w, _ := e.workouts.GetWorkout(ctx, me, s.WorkoutID)
		for _, ex := range w.Exercises {
			if ex.Name == "Curl" && (ex.Reps != 12 || ex.Weight != 30 || ex.Sets != 3) {
				t.Errorf("Curl = %+v, want 3 sets of 12 @ 30 (the logged set)", ex)
			}
		}
	})
}

func TestFinish_ExerciseWithAllSetsDeletedKeepsItsPlan(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e)
		for _, set := range s.Exercises[0].Sets {
			e.sessions.DeleteExerciseSet(ctx, me, set.ID)
		}
		if c := mustSummary(t, e, me, s.ID).Changes; len(c.SetCounts) != 0 {
			t.Errorf("zero-set exercise reported as a change: %+v", c.SetCounts)
		}
		e.sessions.FinishSession(ctx, me, s.ID, true)
		w, _ := e.workouts.GetWorkout(ctx, me, s.WorkoutID)
		if w.Exercises[0].Sets != 2 {
			t.Errorf("sets = %d, want the plan's 2", w.Exercises[0].Sets)
		}
	})
}

// started_at is written as the server's local wall clock; the duration must not
// be off by the zone's offset.
func TestSummary_DurationIgnoresServerTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/Vancouver")
	if err != nil {
		t.Skip("no tzdata")
	}
	old := time.Local
	time.Local = loc
	defer func() { time.Local = old }()
	withDB(t, func(t *testing.T, e *env) {
		me, s := startPush(t, e)
		sum := mustSummary(t, e, me, s.ID)
		if sum.DurationSeconds > 60 {
			t.Errorf("duration = %ds for a session just started", sum.DurationSeconds)
		}
	})
}

func TestCreateExercise_ConcurrentPositionsAreDistinct(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w, _ := e.workouts.CreateWorkout(ctx, me, "W")
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				e.workouts.CreateExercise(ctx, me, &models.Exercise{Name: string(rune('A' + i)), Sets: 1, Reps: 1, WorkoutID: w.ID})
			}(i)
		}
		wg.Wait()
		if n := e.count(t, `SELECT COUNT(DISTINCT position) FROM exercises WHERE workout_id = $1`, w.ID); n != 8 {
			t.Errorf("%d distinct positions for 8 exercises", n)
		}
	})
}
