package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"

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
		if n := e.count(t, `SELECT COUNT(*) FROM routines`); n != 0 {
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

		// A valid update replaces name and workouts together.
		if err := e.routines.UpdateRoutine(ctx, me, r.ID, "Renamed", "", []string{b.ID}); err != nil {
			t.Fatal(err)
		}
		got, _ = e.routines.GetRoutine(ctx, me, r.ID)
		if ids := routineWorkoutIDs(t, e, me, r.ID); got.Name != "Renamed" || len(ids) != 1 || ids[0] != b.ID {
			t.Errorf("after update: name %q workouts %v, want Renamed [B]", got.Name, ids)
		}
		// nil workouts leaves them alone.
		if err := e.routines.UpdateRoutine(ctx, me, r.ID, "Again", "", nil); err != nil {
			t.Fatal(err)
		}
		if ids := routineWorkoutIDs(t, e, me, r.ID); len(ids) != 1 {
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
		if n := e.count(t, `SELECT COUNT(*) FROM routines`); n != 8 {
			t.Errorf("%d routines, want 8", n)
		}
	})
}
