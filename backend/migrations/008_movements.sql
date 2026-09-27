-- Exercise catalog ("movements") and history that survives workout edits.
--
-- A movement is a user's exercise identity ("Barbell Squat"), shared by every
-- workout that uses it, so history, "previous" values and progress follow the
-- movement rather than one workout's copy. Sessions keep their own copy of the
-- names they were performed under, and no longer depend on the workout or its
-- exercise rows: deleting either used to cascade-delete the logged history.

CREATE TABLE IF NOT EXISTS movements (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(50) NOT NULL DEFAULT '',
    rest_seconds INTEGER NOT NULL DEFAULT 90,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- One movement per name per user, ignoring case and surrounding spaces.
CREATE UNIQUE INDEX IF NOT EXISTS idx_movements_user_name ON movements (user_id, LOWER(BTRIM(name)));

ALTER TABLE exercises ADD COLUMN IF NOT EXISTS movement_id VARCHAR(36) REFERENCES movements(id);
ALTER TABLE session_exercises ADD COLUMN IF NOT EXISTS movement_id VARCHAR(36) REFERENCES movements(id);
ALTER TABLE session_exercises ADD COLUMN IF NOT EXISTS name VARCHAR(255);
ALTER TABLE workout_sessions ADD COLUMN IF NOT EXISTS workout_name VARCHAR(255);

-- Backfill: one movement per user for each distinct exercise name (the earliest
-- spelling wins), then link every exercise to it.
INSERT INTO movements (id, user_id, name, created_at, updated_at)
SELECT gen_random_uuid()::text, first.user_id, first.name, first.created_at, first.created_at
FROM (
    SELECT DISTINCT ON (w.user_id, LOWER(BTRIM(e.name)))
        w.user_id, BTRIM(e.name) AS name, e.created_at
    FROM exercises e
    JOIN workouts w ON w.id = e.workout_id
    ORDER BY w.user_id, LOWER(BTRIM(e.name)), e.created_at, e.id
) first
ON CONFLICT DO NOTHING;

UPDATE exercises e
SET movement_id = m.id
FROM workouts w, movements m
WHERE w.id = e.workout_id
  AND m.user_id = w.user_id
  AND LOWER(BTRIM(m.name)) = LOWER(BTRIM(e.name))
  AND e.movement_id IS NULL;

ALTER TABLE exercises ALTER COLUMN movement_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_exercises_movement_id ON exercises(movement_id);

-- Sessions: copy the movement and the names they were performed under.
UPDATE session_exercises se
SET movement_id = e.movement_id, name = e.name
FROM exercises e
WHERE e.id = se.exercise_id AND se.movement_id IS NULL;

UPDATE workout_sessions ws
SET workout_name = w.name
FROM workouts w
WHERE w.id = ws.workout_id AND ws.workout_name IS NULL;

ALTER TABLE session_exercises ALTER COLUMN movement_id SET NOT NULL;
ALTER TABLE session_exercises ALTER COLUMN name SET NOT NULL;
UPDATE workout_sessions SET workout_name = '' WHERE workout_name IS NULL;
ALTER TABLE workout_sessions ALTER COLUMN workout_name SET DEFAULT '';
ALTER TABLE workout_sessions ALTER COLUMN workout_name SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_session_exercises_movement_id ON session_exercises(movement_id);
-- Deleting an exercise clears session_exercises.exercise_id (ON DELETE SET NULL).
CREATE INDEX IF NOT EXISTS idx_session_exercises_exercise_id ON session_exercises(exercise_id);

-- History no longer depends on the plan: deleting a workout or one of its
-- exercises leaves logged sessions alone (the link is cleared instead).
ALTER TABLE session_exercises DROP CONSTRAINT IF EXISTS session_exercises_exercise_id_fkey;
ALTER TABLE session_exercises ALTER COLUMN exercise_id DROP NOT NULL;
ALTER TABLE session_exercises
    ADD CONSTRAINT session_exercises_exercise_id_fkey
    FOREIGN KEY (exercise_id) REFERENCES exercises(id) ON DELETE SET NULL;

ALTER TABLE workout_sessions DROP CONSTRAINT IF EXISTS workout_sessions_workout_id_fkey;
ALTER TABLE workout_sessions ALTER COLUMN workout_id DROP NOT NULL;
ALTER TABLE workout_sessions
    ADD CONSTRAINT workout_sessions_workout_id_fkey
    FOREIGN KEY (workout_id) REFERENCES workouts(id) ON DELETE SET NULL;
