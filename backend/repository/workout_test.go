package repository_test

import (
	"context"
	"errors"
	"testing"

	"liftoff/backend/repository"
)

func TestUpdateExercise_PlanAndOrder(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, other := e.user(t, "me@x.com"), e.user(t, "other@x.com")
		w := e.workout(t, me, "Push", 3, 3, 3, 3)
		ids := make([]string, len(w.Exercises))
		for i, ex := range w.Exercises {
			ids[i] = ex.ID
		}
		names := func() []string {
			list, err := e.workouts.GetExercisesByWorkout(ctx, w.ID)
			if err != nil {
				t.Fatal(err)
			}
			var out []string
			for _, ex := range list {
				out = append(out, ex.ID)
			}
			return out
		}
		same := func(a, b []string) bool {
			if len(a) != len(b) {
				return false
			}
			for i := range a {
				if a[i] != b[i] {
					return false
				}
			}
			return true
		}
		i, f := 5, 62.5

		// Partial update changes only what's sent.
		got, err := e.workouts.UpdateExercise(ctx, me, ids[0], repository.ExercisePatch{Sets: &i, Weight: &f})
		if err != nil {
			t.Fatal(err)
		}
		if got.Sets != 5 || got.Reps != 8 || got.Weight != 62.5 {
			t.Errorf("updated = %+v", got)
		}

		// Move last to the top, then to the end (clamped), after a delete leaves a gap.
		zero, far := 0, 99
		if _, err := e.workouts.UpdateExercise(ctx, me, ids[3], repository.ExercisePatch{Position: &zero}); err != nil {
			t.Fatal(err)
		}
		if want := []string{ids[3], ids[0], ids[1], ids[2]}; !same(names(), want) {
			t.Errorf("after move to top = %v", names())
		}
		if err := e.workouts.DeleteExercise(ctx, me, ids[0]); err != nil {
			t.Fatal(err)
		}
		if _, err := e.workouts.UpdateExercise(ctx, me, ids[3], repository.ExercisePatch{Position: &far}); err != nil {
			t.Fatal(err)
		}
		if want := []string{ids[1], ids[2], ids[3]}; !same(names(), want) {
			t.Errorf("after move to end = %v", names())
		}
		if n := e.count(t, `SELECT COUNT(DISTINCT position) FROM exercises WHERE workout_id = $1`, w.ID); n != 3 {
			t.Errorf("positions not contiguous: %d distinct", n)
		}

		// Another user's exercise, or a missing one, is not found.
		if _, err := e.workouts.UpdateExercise(ctx, other, ids[1], repository.ExercisePatch{Sets: &i}); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("other user's exercise: err = %v", err)
		}
		if _, err := e.workouts.UpdateExercise(ctx, me, "nope", repository.ExercisePatch{Sets: &i}); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("missing exercise: err = %v", err)
		}
	})
}
