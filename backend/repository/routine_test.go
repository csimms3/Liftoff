package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"liftoff/backend/models"
	"liftoff/backend/repository"
)

func routineWorkoutIDs(t *testing.T, e *env, userID, routineID string) []string {
	t.Helper()
	r, err := e.routines.GetRoutine(context.Background(), userID, routineID)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, rw := range r.Workouts {
		ids = append(ids, rw.WorkoutID)
	}
	return ids
}

func TestCreateRoutine_WithWorkoutsInOrder(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		me := e.user(t, "me@example.com")
		a, b := e.workout(t, me, "A"), e.workout(t, me, "B")

		r, err := e.routines.CreateRoutine(context.Background(), me, "Split", "", []string{b.ID, a.ID})
		if err != nil {
			t.Fatal(err)
		}
		if got := routineWorkoutIDs(t, e, me, r.ID); len(got) != 2 || got[0] != b.ID || got[1] != a.ID {
			t.Errorf("workouts = %v, want [B A]", got)
		}
	})
}

func TestCreateRoutine_OtherUsersWorkoutWritesNothing(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		me := e.user(t, "me@example.com")
		other := e.user(t, "other@example.com")
		mine, theirs := e.workout(t, me, "Mine"), e.workout(t, other, "Theirs")

		_, err := e.routines.CreateRoutine(context.Background(), me, "Split", "", []string{mine.ID, theirs.ID})
		if !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		if n := e.count(t, `SELECT COUNT(*) FROM routines WHERE name = 'Split'`); n != 0 {
			t.Errorf("%d routines written", n)
		}
	})
}

func TestUpdateRoutine_RejectedChangeLeavesRoutineIntact(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		other := e.user(t, "other@example.com")
		a, b := e.workout(t, me, "A"), e.workout(t, me, "B")
		theirs := e.workout(t, other, "Theirs")
		r, err := e.routines.CreateRoutine(ctx, me, "Split", "", []string{a.ID, b.ID})
		if err != nil {
			t.Fatal(err)
		}

		err = e.routines.UpdateRoutine(ctx, me, r.ID, "Renamed", "", []string{b.ID, theirs.ID})
		if !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		got, _ := e.routines.GetRoutine(ctx, me, r.ID)
		if got.Name != "Split" {
			t.Errorf("name = %q, want unchanged (the whole update must roll back)", got.Name)
		}
		if ids := routineWorkoutIDs(t, e, me, r.ID); len(ids) != 2 || ids[0] != a.ID {
			t.Errorf("workouts = %v, want unchanged [A B]", ids)
		}

		// A valid update changes name and order together.
		if err := e.routines.UpdateRoutine(ctx, me, r.ID, "Renamed", "", []string{b.ID}); err != nil {
			t.Fatal(err)
		}
		got, _ = e.routines.GetRoutine(ctx, me, r.ID)
		if ids := routineWorkoutIDs(t, e, me, r.ID); got.Name != "Renamed" || len(ids) != 2 || ids[0] != b.ID || ids[1] != a.ID {
			t.Errorf("after update: name %q workouts %v, want Renamed [B A]", got.Name, ids)
		}
		// nil workouts leaves them alone.
		if err := e.routines.UpdateRoutine(ctx, me, r.ID, "Again", "", nil); err != nil {
			t.Fatal(err)
		}
		if ids := routineWorkoutIDs(t, e, me, r.ID); len(ids) != 2 || ids[0] != b.ID {
			t.Errorf("nil workoutIDs changed the workouts: %v", ids)
		}
	})
}

func TestUpdateRoutine_OtherUsersRoutine(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		owner := e.user(t, "owner@example.com")
		other := e.user(t, "other@example.com")
		r, err := e.routines.CreateRoutine(ctx, owner, "Mine", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.routines.UpdateRoutine(ctx, other, r.ID, "Hijacked", "", nil); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		got, _ := e.routines.GetRoutine(ctx, owner, r.ID)
		if got.Name != "Mine" {
			t.Errorf("name = %q, want Mine", got.Name)
		}
	})
}

func TestCreateFromTemplate_CreatesRoutineWithWorkouts(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		me := e.user(t, "me@example.com")
		tpls := e.routines.GetRoutineTemplates()
		r, err := e.routines.CreateFromTemplate(context.Background(), me, tpls[0].ID, "")
		if err != nil {
			t.Fatal(err)
		}
		if ids := routineWorkoutIDs(t, e, me, r.ID); len(ids) != len(tpls[0].Workouts) {
			t.Errorf("%d workouts, want %d", len(ids), len(tpls[0].Workouts))
		}
		if _, err := e.routines.CreateFromTemplate(context.Background(), me, "nope", ""); !errors.Is(err, repository.ErrTemplateNotFound) {
			t.Errorf("unknown template: %v, want ErrTemplateNotFound", err)
		}
	})
}

// Concurrent read-then-write transactions all succeed.
func TestCreateRoutine_Concurrent(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "A")
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := e.routines.CreateRoutine(context.Background(), me, "R", "", []string{w.ID}); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		if n := e.count(t, `SELECT COUNT(*) FROM routines WHERE name = 'R'`); n != 8 {
			t.Errorf("%d routines, want 8", n)
		}
	})
}

func (e *env) currentRoutine(t *testing.T, userID string) string {
	t.Helper()
	id, err := e.routines.GetCurrentRoutineID(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCreateWorkout_DefaultsToCurrentRoutine(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")

		// No routine at all: "My Workouts" is created, made current and used.
		w1, err := e.workouts.CreateWorkout(ctx, me, "First", "")
		if err != nil {
			t.Fatal(err)
		}
		cur := e.currentRoutine(t, me)
		if cur == "" || w1.RoutineID != cur {
			t.Fatalf("routine_id = %q, current = %q", w1.RoutineID, cur)
		}
		r, err := e.routines.GetRoutine(ctx, me, cur)
		if err != nil || r.Name != "My Workouts" {
			t.Fatalf("routine = %+v, %v", r, err)
		}

		// A second workout reuses it, after the first.
		w2, _ := e.workouts.CreateWorkout(ctx, me, "Second", "")
		if w2.RoutineID != cur {
			t.Errorf("second workout in %q, want %q", w2.RoutineID, cur)
		}
		if n := e.count(t, `SELECT COUNT(*) FROM routines WHERE user_id = $1`, me); n != 1 {
			t.Errorf("%d routines, want 1", n)
		}
		if ids := routineWorkoutIDs(t, e, me, cur); len(ids) != 2 || ids[0] != w1.ID || ids[1] != w2.ID {
			t.Errorf("order = %v, want [First Second]", ids)
		}

		// Another routine, made current, is the new default; an explicit one overrides.
		other, _ := e.routines.CreateRoutine(ctx, me, "Other", "", nil)
		if err := e.routines.SetCurrentRoutine(ctx, me, other.ID); err != nil {
			t.Fatal(err)
		}
		w3, _ := e.workouts.CreateWorkout(ctx, me, "Third", "")
		w4, _ := e.workouts.CreateWorkout(ctx, me, "Fourth", cur)
		if w3.RoutineID != other.ID || w4.RoutineID != cur {
			t.Errorf("third in %q (want %q), fourth in %q (want %q)", w3.RoutineID, other.ID, w4.RoutineID, cur)
		}
		got, _ := e.workouts.GetWorkout(ctx, me, w3.ID)
		if got.RoutineID != other.ID {
			t.Errorf("GetWorkout routine_id = %q", got.RoutineID)
		}
	})
}

func TestCreateWorkout_RejectsOtherUsersRoutine(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		other := e.user(t, "other@example.com")
		theirs, err := e.routines.CreateRoutine(ctx, other, "Theirs", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.workouts.CreateWorkout(ctx, me, "Sneaky", theirs.ID); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		if _, err := e.workouts.CreateWorkout(ctx, me, "Nowhere", "no-such-routine"); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("unknown routine: err = %v, want ErrNotFound", err)
		}
		if n := e.count(t, `SELECT COUNT(*) FROM workouts`); n != 0 {
			t.Errorf("%d workouts written", n)
		}
		if err := e.routines.SetCurrentRoutine(ctx, me, theirs.ID); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("SetCurrentRoutine err = %v, want ErrNotFound", err)
		}
		if got := e.currentRoutine(t, me); got != "" {
			t.Errorf("current = %q, want none", got)
		}
		if err := e.routines.DeleteRoutine(ctx, me, theirs.ID); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("DeleteRoutine err = %v, want ErrNotFound", err)
		}
		if n := e.count(t, `SELECT COUNT(*) FROM routines WHERE id = $1`, theirs.ID); n != 1 {
			t.Error("other user's routine was deleted")
		}
	})
}

func TestDeleteRoutine_CurrentFallsBack(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		first, _ := e.routines.CreateRoutine(ctx, me, "First", "", nil)
		second, _ := e.routines.CreateRoutine(ctx, me, "Second", "", nil)
		third, _ := e.routines.CreateRoutine(ctx, me, "Third", "", nil)
		// The first routine created became current.
		if got := e.currentRoutine(t, me); got != first.ID {
			t.Fatalf("current = %q, want the first routine", got)
		}

		// Deleting a non-current routine leaves current alone.
		if err := e.routines.DeleteRoutine(ctx, me, third.ID); err != nil {
			t.Fatal(err)
		}
		if got := e.currentRoutine(t, me); got != first.ID {
			t.Errorf("current = %q, want unchanged", got)
		}
		// Deleting the current one falls back to another of theirs.
		if err := e.routines.DeleteRoutine(ctx, me, first.ID); err != nil {
			t.Fatal(err)
		}
		if got := e.currentRoutine(t, me); got != second.ID {
			t.Errorf("current = %q, want %q", got, second.ID)
		}
		// Deleting the last one leaves none.
		if err := e.routines.DeleteRoutine(ctx, me, second.ID); err != nil {
			t.Fatal(err)
		}
		if got := e.currentRoutine(t, me); got != "" {
			t.Errorf("current = %q, want none", got)
		}
		// A new workout recreates a default routine.
		w, err := e.workouts.CreateWorkout(ctx, me, "Again", "")
		if err != nil || w.RoutineID == "" || e.currentRoutine(t, me) != w.RoutineID {
			t.Errorf("workout %+v, err %v", w, err)
		}
	})
}

func TestDeleteRoutine_DeletesWorkoutsKeepsHistory(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		r, _ := e.routines.CreateRoutine(ctx, me, "Split", "", nil)
		keep, _ := e.routines.CreateRoutine(ctx, me, "Keep", "", nil)
		w, err := e.workouts.CreateWorkout(ctx, me, "Push Day", r.ID)
		if err != nil {
			t.Fatal(err)
		}
		ex := &models.Exercise{Name: "Bench Press", Sets: 3, Reps: 8, Weight: 60, WorkoutID: w.ID}
		if err := e.workouts.CreateExercise(ctx, me, ex); err != nil {
			t.Fatal(err)
		}
		survivor, _ := e.workouts.CreateWorkout(ctx, me, "Survivor", keep.ID)

		sess, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		e.exec(t, `UPDATE workout_sessions SET is_active = false, ended_at = NOW() WHERE id = $1`, sess.ID)

		if err := e.routines.DeleteRoutine(ctx, me, r.ID); err != nil {
			t.Fatal(err)
		}
		if n := e.count(t, `SELECT COUNT(*) FROM workouts WHERE id = $1`, w.ID); n != 0 {
			t.Error("workout survived its routine")
		}
		if n := e.count(t, `SELECT COUNT(*) FROM exercises WHERE id = $1`, ex.ID); n != 0 {
			t.Error("exercise survived its workout")
		}
		if n := e.count(t, `SELECT COUNT(*) FROM workouts WHERE id = $1`, survivor.ID); n != 1 {
			t.Error("another routine's workout was deleted")
		}
		if n := e.count(t, `SELECT COUNT(*) FROM workout_sessions WHERE id = $1 AND workout_id IS NULL AND workout_name = 'Push Day'`, sess.ID); n != 1 {
			t.Error("completed session should be kept with its workout name and a cleared link")
		}
		if n := e.count(t, `SELECT COUNT(*) FROM session_exercises WHERE session_id = $1 AND exercise_id IS NULL AND name = 'Bench Press'`, sess.ID); n != 1 {
			t.Error("session exercise should be kept with its name and a cleared link")
		}
	})
}

func TestUpdateRoutine_MovesWorkoutsBetweenRoutines(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		a, b, c := e.workout(t, me, "A"), e.workout(t, me, "B"), e.workout(t, me, "C")
		// All three start in the auto-created "My Workouts".
		home := a.RoutineID
		target, err := e.routines.CreateRoutine(ctx, me, "Target", "", []string{c.ID})
		if err != nil {
			t.Fatal(err)
		}
		if ids := routineWorkoutIDs(t, e, me, home); len(ids) != 2 || ids[0] != a.ID || ids[1] != b.ID {
			t.Fatalf("home = %v, want [A B] after C moved out", ids)
		}

		// Move B in front of C; unlisted C stays after it. A is untouched.
		if err := e.routines.UpdateRoutine(ctx, me, target.ID, "Target", "", []string{b.ID}); err != nil {
			t.Fatal(err)
		}
		if ids := routineWorkoutIDs(t, e, me, target.ID); len(ids) != 2 || ids[0] != b.ID || ids[1] != c.ID {
			t.Errorf("target = %v, want [B C]", ids)
		}
		if ids := routineWorkoutIDs(t, e, me, home); len(ids) != 1 || ids[0] != a.ID {
			t.Errorf("home = %v, want [A]", ids)
		}
		got, _ := e.workouts.GetWorkout(ctx, me, b.ID)
		if got.RoutineID != target.ID {
			t.Errorf("B routine_id = %q", got.RoutineID)
		}
		// Reorder, with a repeated id ignored.
		if err := e.routines.UpdateRoutine(ctx, me, target.ID, "Target", "", []string{c.ID, b.ID, c.ID}); err != nil {
			t.Fatal(err)
		}
		if ids := routineWorkoutIDs(t, e, me, target.ID); len(ids) != 2 || ids[0] != c.ID || ids[1] != b.ID {
			t.Errorf("target = %v, want [C B]", ids)
		}
	})
}

// GET /api/workouts lists workouts by routine (oldest routine first), then by
// their position in it, not by creation time.
func TestGetWorkouts_OrderedByRoutineThenPosition(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		first, _ := e.routines.CreateRoutine(ctx, me, "First", "", nil)
		second, _ := e.routines.CreateRoutine(ctx, me, "Second", "", nil)
		// Created interleaved across the routines.
		b1, _ := e.workouts.CreateWorkout(ctx, me, "B1", second.ID)
		a1, _ := e.workouts.CreateWorkout(ctx, me, "A1", first.ID)
		b2, _ := e.workouts.CreateWorkout(ctx, me, "B2", second.ID)
		a2, _ := e.workouts.CreateWorkout(ctx, me, "A2", first.ID)
		// Reorder the first routine: A2 before A1.
		if err := e.routines.UpdateRoutine(ctx, me, first.ID, "First", "", []string{a2.ID, a1.ID}); err != nil {
			t.Fatal(err)
		}

		got, err := e.workouts.GetWorkouts(ctx, me)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{a2.ID, a1.ID, b1.ID, b2.ID}
		if len(got) != len(want) {
			t.Fatalf("%d workouts, want %d", len(got), len(want))
		}
		for i, w := range got {
			if w.ID != want[i] {
				t.Errorf("position %d = %s, want %s", i, w.Name, want[i])
			}
		}
	})
}
