-- A workout belongs to exactly one routine (routine -> workouts is one-to-many),
-- replacing the many-to-many routine_workouts link table, and each user has a
-- current routine.
--
-- Deleting a routine deletes its workouts. Logged history survives that: sessions
-- and their exercises keep their own name snapshots and only have their
-- workout/exercise link cleared (ON DELETE SET NULL, migration 008).

ALTER TABLE workouts ADD COLUMN IF NOT EXISTS routine_id VARCHAR(36) REFERENCES routines(id) ON DELETE CASCADE;
-- Order of the workout within its routine.
ALTER TABLE workouts ADD COLUMN IF NOT EXISTS position INTEGER NOT NULL DEFAULT 0;

-- A workout linked to several routines keeps its oldest link (and that slot).
UPDATE workouts w
SET routine_id = l.routine_id, position = l.slot_order
FROM (
    SELECT DISTINCT ON (rw.workout_id) rw.workout_id, rw.routine_id, rw.slot_order
    FROM routine_workouts rw
    JOIN routines r ON r.id = rw.routine_id
    JOIN workouts w2 ON w2.id = rw.workout_id AND w2.user_id = r.user_id
    ORDER BY rw.workout_id, rw.created_at, rw.id
) l
WHERE w.id = l.workout_id;

-- Workouts in no routine go into a "My Workouts" routine, one per user. Its id is
-- derived from the user id so the workouts below can be pointed at it.
INSERT INTO routines (id, user_id, name, description)
SELECT DISTINCT md5(w.user_id || ':my-workouts')::uuid::text, w.user_id, 'My Workouts', ''
FROM workouts w
WHERE w.routine_id IS NULL AND w.user_id IS NOT NULL;

UPDATE workouts
SET routine_id = md5(user_id || ':my-workouts')::uuid::text,
    position = 0
WHERE routine_id IS NULL AND user_id IS NOT NULL;

-- Contiguous 1..n positions per routine, keeping the order so far.
UPDATE workouts w
SET position = r.n
FROM (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY routine_id ORDER BY position, created_at, id) AS n
    FROM workouts
) r
WHERE w.id = r.id;

ALTER TABLE workouts ALTER COLUMN routine_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_workouts_routine_id ON workouts(routine_id);

DROP TABLE routine_workouts;

-- The routine the main page shows. Defaults to the user's first (oldest) routine.
ALTER TABLE users ADD COLUMN IF NOT EXISTS current_routine_id VARCHAR(36) REFERENCES routines(id) ON DELETE SET NULL;

UPDATE users u
SET current_routine_id = first.id
FROM (
    SELECT DISTINCT ON (user_id) id, user_id
    FROM routines
    ORDER BY user_id, created_at, id
) first
WHERE first.user_id = u.id AND u.current_routine_id IS NULL;
