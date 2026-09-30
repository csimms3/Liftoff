package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"liftoff/backend/models"
	"liftoff/backend/repository"
)

func order(t *testing.T, e *env, userID string) []string {
	t.Helper()
	s, err := e.sessions.GetActiveSessionWithExercises(context.Background(), userID)
	if err != nil || s == nil {
		t.Fatalf("active session: %v %v", s, err)
	}
	var names []string
	for _, se := range s.Exercises {
		names = append(names, se.Name)
	}
	return names
}

func equal(a, b []string) bool {
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

func startPush(t *testing.T, e *env) (string, *models.WorkoutSession) {
	me := e.user(t, "me@example.com")
	w := e.workout(t, me, "Push", 2, 2) // Push-ex0, Push-ex1
	s, err := e.sessions.StartSession(context.Background(), me, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	return me, s
}

func TestAddMovement_ByNameDefaultsAndAppends(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e)
		se, err := e.sessions.AddMovementToSession(ctx, me, s.ID, "", "Lateral Raise", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(se.Sets) != 3 || se.Sets[0].Reps != 10 || se.Sets[0].Weight != 0 || se.Sets[0].Completed {
			t.Errorf("default plan = %+v, want 3x10 @ 0", se.Sets)
		}
		if got := order(t, e, me); !equal(got, []string{"Push-ex0", "Push-ex1", "Lateral Raise"}) {
			t.Errorf("order = %v", got)
		}
		// Same name again reuses the movement.
		again, _ := e.sessions.AddMovementToSession(ctx, me, s.ID, "", "  lateral raise", nil)
		if again.MovementID != se.MovementID {
			t.Error("same name should reuse the movement")
		}
	})
}

func TestAddMovement_CopiesLastTimeAndInsertsAtPosition(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e)
		// Log 2 sets of Push-ex1 last time (80x6, 85x4), then start again.
		done := true
		for i, wr := range [][2]float64{{80, 6}, {85, 4}} {
			w, r := wr[0], int(wr[1])
			if _, err := e.sessions.PatchExerciseSet(ctx, me, s.Exercises[1].Sets[i].ID, repository.SetPatch{Weight: &w, Reps: &r, Completed: &done}); err != nil {
				t.Fatal(err)
			}
		}
		e.sessions.EndSession(ctx, me, s.ID)
		w2 := e.workout(t, me, "Other", 1)
		s2, err := e.sessions.StartSession(ctx, me, w2.ID)
		if err != nil {
			t.Fatal(err)
		}
		se, err := e.sessions.AddMovementToSession(ctx, me, s2.ID, s.Exercises[1].MovementID, "", ptr(0))
		if err != nil {
			t.Fatal(err)
		}
		if len(se.Sets) != 2 || se.Sets[0].Weight != 80 || se.Sets[0].Reps != 6 || se.Sets[1].Weight != 85 {
			t.Errorf("plan = %+v, want last time's 80x6, 85x4", se.Sets)
		}
		if len(se.Previous) != 2 {
			t.Errorf("previous = %d sets, want 2", len(se.Previous))
		}
		if got := order(t, e, me); got[0] != "Push-ex1" || len(got) != 2 {
			t.Errorf("order = %v, want the new exercise first", got)
		}
	})
}

func ptr(n int) *int { return &n }

func TestRemoveMoveReplace(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e)
		a, _ := e.sessions.AddMovementToSession(ctx, me, s.ID, "", "A", nil)
		b, _ := e.sessions.AddMovementToSession(ctx, me, s.ID, "", "B", nil)
		// Push-ex0, Push-ex1, A, B
		if err := e.sessions.MoveSessionExercise(ctx, me, b.ID, 0); err != nil {
			t.Fatal(err)
		}
		if got := order(t, e, me); !equal(got, []string{"B", "Push-ex0", "Push-ex1", "A"}) {
			t.Errorf("after move to top = %v", got)
		}
		if err := e.sessions.MoveSessionExercise(ctx, me, b.ID, 99); err != nil { // clamps
			t.Fatal(err)
		}
		if got := order(t, e, me); !equal(got, []string{"Push-ex0", "Push-ex1", "A", "B"}) {
			t.Errorf("after move to end = %v", got)
		}
		if err := e.sessions.RemoveSessionExercise(ctx, me, a.ID); err != nil {
			t.Fatal(err)
		}
		if got := order(t, e, me); !equal(got, []string{"Push-ex0", "Push-ex1", "B"}) {
			t.Errorf("after remove = %v", got)
		}
		if n := e.count(t, `SELECT COUNT(*) FROM exercise_sets WHERE session_exercise_id = $1`, a.ID); n != 0 {
			t.Errorf("%d sets left for the removed exercise", n)
		}
		if n := e.count(t, `SELECT COUNT(DISTINCT position) FROM session_exercises`); n != 3 {
			t.Errorf("positions not contiguous: %d distinct", n)
		}

		// Replace keeps sets and position, clears the plan link.
		r, err := e.sessions.ReplaceSessionExercise(ctx, me, s.Exercises[0].ID, "", "Incline Press")
		if err != nil {
			t.Fatal(err)
		}
		if r.Name != "Incline Press" || len(r.Sets) != 2 || r.ExerciseID != "" {
			t.Errorf("replaced = %+v", r)
		}
		if got := order(t, e, me); got[0] != "Incline Press" {
			t.Errorf("replaced exercise lost its position: %v", got)
		}
	})
}

func TestSessionExerciseEdits_Ownership(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e)
		other := e.user(t, "other@example.com")
		otherW := e.workout(t, other, "Theirs", 1)
		theirMovement := otherW.Exercises[0].MovementID
		se := s.Exercises[0]

		nf := func(name string, err error) {
			if !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("%s: %v, want ErrNotFound", name, err)
			}
		}
		_, err := e.sessions.AddMovementToSession(ctx, other, s.ID, "", "X", nil)
		nf("add to another user's session", err)
		_, err = e.sessions.AddMovementToSession(ctx, me, s.ID, theirMovement, "", nil)
		nf("add another user's movement", err)
		nf("remove", e.sessions.RemoveSessionExercise(ctx, other, se.ID))
		nf("move", e.sessions.MoveSessionExercise(ctx, other, se.ID, 0))
		_, err = e.sessions.ReplaceSessionExercise(ctx, other, se.ID, "", "X")
		nf("replace", err)
		_, err = e.sessions.ReplaceSessionExercise(ctx, me, se.ID, theirMovement, "")
		nf("replace with another user's movement", err)

		// Ended sessions can't be edited.
		e.sessions.EndSession(ctx, me, s.ID)
		nf("remove from ended session", e.sessions.RemoveSessionExercise(ctx, me, se.ID))
		_, err = e.sessions.AddMovementToSession(ctx, me, s.ID, "", "X", nil)
		nf("add to ended session", err)
	})
}

func TestListMovements_RecentFirstPerUser(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e)
		e.sessions.AddMovementToSession(ctx, me, s.ID, "", "Zed", nil)
		e.workout(t, e.user(t, "other@example.com"), "Theirs", 1)
		got, err := e.sessions.ListMovements(ctx, me)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("%d movements, want 3 (others' hidden)", len(got))
		}
		for _, m := range got {
			if m.LastUsed == nil {
				t.Errorf("%s never used but is in the session", m.Name)
			}
		}
	})
}

func TestAddMovement_ConcurrentKeepsPositionsDistinct(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me, s := startPush(t, e)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if _, err := e.sessions.AddMovementToSession(ctx, me, s.ID, "", string(rune('A'+i)), nil); err != nil {
					t.Error(err)
				}
			}(i)
		}
		wg.Wait()
		if n := e.count(t, `SELECT COUNT(DISTINCT position) FROM session_exercises`); n != 10 {
			t.Errorf("%d distinct positions for 10 exercises", n)
		}
	})
}

func TestExerciseNameValidation(t *testing.T) {
	long := make([]rune, 256)
	for i := range long {
		long[i] = 'x'
	}
	for name, want := range map[string]bool{"Squat": true, "  Squat ": true, "": false, "   ": false, string(long[:255]): true, string(long): false} {
		if got := repository.ValidExerciseName(name); got != want {
			t.Errorf("ValidExerciseName(%d chars) = %v, want %v", len(name), got, want)
		}
	}
}
