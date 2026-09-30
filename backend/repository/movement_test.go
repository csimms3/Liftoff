package repository_test

import (
	"context"
	"sync"
	"testing"

	"liftoff/backend/models"
	"liftoff/backend/repository"
)

// logFirstSet completes set 1 of the first exercise with the given values.
func logFirstSet(t *testing.T, e *env, userID string, s *models.WorkoutSession, weight float64, reps int) {
	t.Helper()
	done := true
	if _, err := e.sessions.PatchExerciseSet(context.Background(), userID, s.Exercises[0].Sets[0].ID,
		repository.SetPatch{Weight: &weight, Reps: &reps, Completed: &done}); err != nil {
		t.Fatal(err)
	}
}

func workoutWith(t *testing.T, e *env, userID, name, exercise string) *models.Workout {
	t.Helper()
	ctx := context.Background()
	w, err := e.workouts.CreateWorkout(ctx, userID, name, "")
	if err != nil {
		t.Fatal(err)
	}
	ex := &models.Exercise{Name: exercise, Sets: 2, Reps: 8, Weight: 60, WorkoutID: w.ID}
	if err := e.workouts.CreateExercise(ctx, userID, ex); err != nil {
		t.Fatal(err)
	}
	w, err = e.workouts.GetWorkout(ctx, userID, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestMovements_SameNameAcrossWorkoutsIsOneMovement(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		me := e.user(t, "me@example.com")
		other := e.user(t, "other@example.com")
		a := workoutWith(t, e, me, "Push A", "Bench Press")
		b := workoutWith(t, e, me, "Push B", "  bench press ")
		theirs := workoutWith(t, e, other, "Theirs", "Bench Press")

		if a.Exercises[0].MovementID == "" || a.Exercises[0].MovementID != b.Exercises[0].MovementID {
			t.Errorf("same name (case/space-insensitive) should share a movement: %q vs %q",
				a.Exercises[0].MovementID, b.Exercises[0].MovementID)
		}
		if theirs.Exercises[0].MovementID == a.Exercises[0].MovementID {
			t.Error("another user's exercise must not share my movement")
		}
		if n := e.count(t, `SELECT COUNT(*) FROM movements WHERE user_id = $1`, me); n != 1 {
			t.Errorf("%d movements, want 1", n)
		}
	})
}

// "Previous" follows the movement, whichever workout it was last done in.
func TestPrevious_AcrossWorkouts(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		a := workoutWith(t, e, me, "Push A", "Bench Press")
		b := workoutWith(t, e, me, "Push B", "bench press")

		s, err := e.sessions.StartSession(ctx, me, a.ID)
		if err != nil {
			t.Fatal(err)
		}
		logFirstSet(t, e, me, s, 100, 5)

		s2, err := e.sessions.StartSession(ctx, me, b.ID) // ends the first
		if err != nil {
			t.Fatal(err)
		}
		p := s2.Exercises[0].Previous
		if len(p) != 1 || p[0].Weight != 100 || p[0].Reps != 5 {
			t.Errorf("previous in workout B = %v, want [100x5] from workout A", p)
		}
	})
}

// Deleting a workout (or an exercise from it) no longer deletes logged history.
func TestHistory_SurvivesDeletingWorkout(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := workoutWith(t, e, me, "Legs", "Squat")
		s, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		logFirstSet(t, e, me, s, 140, 5)
		if _, err := e.sessions.EndSession(ctx, me, s.ID); err != nil {
			t.Fatal(err)
		}

		if err := e.workouts.DeleteWorkout(ctx, me, w.ID); err != nil {
			t.Fatal(err)
		}

		history, err := e.sessions.GetCompletedSessions(ctx, me)
		if err != nil || len(history) != 1 {
			t.Fatalf("history = %d sessions (err %v), want the session kept", len(history), err)
		}
		if history[0].Workout == nil || history[0].Workout.Name != "Legs" {
			t.Errorf("history workout = %+v, want name Legs kept", history[0].Workout)
		}
		if n := e.count(t, `SELECT COUNT(*) FROM exercise_sets WHERE completed`); n != 1 {
			t.Errorf("%d logged sets, want 1 kept", n)
		}
		progress, err := e.sessions.GetProgressData(ctx, me)
		if err != nil || len(progress) != 1 || progress[0]["exerciseName"] != "Squat" {
			t.Errorf("progress = %v (err %v), want Squat kept", progress, err)
		}
	})
}

// The history list used to show "Unknown Workout" for every session.
func TestHistory_IncludesWorkoutName(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := workoutWith(t, e, me, "Pull", "Row")
		s, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.sessions.EndSession(ctx, me, s.ID); err != nil {
			t.Fatal(err)
		}
		history, _ := e.sessions.GetCompletedSessions(ctx, me)
		if len(history) != 1 || history[0].Workout == nil || history[0].Workout.Name != "Pull" || history[0].Workout.ID != w.ID {
			t.Errorf("history = %+v, want workout Pull", history)
		}
	})
}

// An active session keeps working when its workout or an exercise is deleted mid-session.
func TestActiveSession_SurvivesDeletingItsWorkout(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := e.workout(t, me, "Push", 2, 2)
		if _, err := e.sessions.StartSession(ctx, me, w.ID); err != nil {
			t.Fatal(err)
		}
		if err := e.workouts.DeleteExercise(ctx, me, w.Exercises[1].ID); err != nil {
			t.Fatal(err)
		}
		if err := e.workouts.DeleteWorkout(ctx, me, w.ID); err != nil {
			t.Fatal(err)
		}

		s, err := e.sessions.GetActiveSessionWithExercises(ctx, me)
		if err != nil || s == nil {
			t.Fatalf("active session after deleting its workout: %v, %v", s, err)
		}
		if s.Workout == nil || s.Workout.Name != "Push" || s.WorkoutID != "" {
			t.Errorf("workout = %+v (id %q), want name kept and no id", s.Workout, s.WorkoutID)
		}
		if len(s.Exercises) != 2 {
			t.Fatalf("%d exercises, want both kept", len(s.Exercises))
		}
		for _, se := range s.Exercises {
			if se.Exercise == nil || se.Exercise.Name == "" || len(se.Sets) != 2 {
				t.Errorf("session exercise %+v lost its name or sets", se)
			}
		}
	})
}

// A movement listed twice (e.g. top sets and back-off sets): each row gets the
// matching row's sets from last time, not the same ones.
func TestPrevious_RepeatedMovementPairsByOccurrence(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w, err := e.workouts.CreateWorkout(ctx, me, "Bench day", "")
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"Bench Press", "BENCH PRESS "} { // same movement
			if err := e.workouts.CreateExercise(ctx, me, &models.Exercise{Name: name, Sets: 1, Reps: 5, Weight: 60, WorkoutID: w.ID}); err != nil {
				t.Fatal(err)
			}
		}
		s, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		done := true
		for i, weight := range []float64{90, 70} { // top set, back-off set
			wt := weight
			if _, err := e.sessions.PatchExerciseSet(ctx, me, s.Exercises[i].Sets[0].ID, repository.SetPatch{Weight: &wt, Completed: &done}); err != nil {
				t.Fatal(err)
			}
		}
		s2, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		for i, want := range []float64{90, 70} {
			p := s2.Exercises[i].Previous
			if len(p) != 1 || p[0].Weight != want {
				t.Errorf("row %d previous = %v, want [%v]", i+1, p, want)
			}
		}
	})
}

func TestCreateExercise_BlankNameCreatesNothing(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w, err := e.workouts.CreateWorkout(ctx, me, "W", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.workouts.CreateExercise(ctx, me, &models.Exercise{Name: "   ", Sets: 1, Reps: 1, WorkoutID: w.ID}); err == nil {
			t.Error("want an error for a blank name")
		}
		if n := e.count(t, `SELECT COUNT(*) FROM movements`) + e.count(t, `SELECT COUNT(*) FROM exercises`); n != 0 {
			t.Errorf("%d rows written for a blank name", n)
		}
	})
}

func TestCreateExercise_ConcurrentSameNewNameIsOneMovement(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		var workouts []*models.Workout
		for i := 0; i < 4; i++ {
			w, err := e.workouts.CreateWorkout(ctx, me, "W", "")
			if err != nil {
				t.Fatal(err)
			}
			workouts = append(workouts, w)
		}
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				name := []string{"Deadlift", "deadlift", " Deadlift", "DEADLIFT "}[i%4]
				if err := e.workouts.CreateExercise(ctx, me, &models.Exercise{Name: name, Sets: 1, Reps: 1, WorkoutID: workouts[i%4].ID}); err != nil {
					t.Error(err)
				}
			}(i)
		}
		wg.Wait()
		if n := e.count(t, `SELECT COUNT(*) FROM movements`); n != 1 {
			t.Errorf("%d movements, want 1", n)
		}
	})
}

// Quick log without an active session: the planned row stays empty and the set
// goes in an extra row after it. Next time, that set is still "previous".
func TestPrevious_QuickLogShape(t *testing.T) {
	withDB(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		me := e.user(t, "me@example.com")
		w := workoutWith(t, e, me, "Pull", "Deadlift")
		s, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		extra, err := e.sessions.CreateSessionExercise(ctx, me, s.ID, w.Exercises[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		set := &models.ExerciseSet{SessionExerciseID: extra.ID, Reps: 5, Weight: 150, Completed: true}
		if err := e.sessions.CreateExerciseSet(ctx, me, set); err != nil {
			t.Fatal(err)
		}
		if _, err := e.sessions.EndSession(ctx, me, s.ID); err != nil {
			t.Fatal(err)
		}

		s2, err := e.sessions.StartSession(ctx, me, w.ID)
		if err != nil {
			t.Fatal(err)
		}
		if p := s2.Exercises[0].Previous; len(p) != 1 || p[0].Weight != 150 {
			t.Errorf("previous = %v, want the quick-logged [150x5]", p)
		}
	})
}
