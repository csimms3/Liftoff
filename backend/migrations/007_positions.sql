-- Explicit order for session exercises and their sets. Rows created together
-- share a created_at, so ordering by it was arbitrary (and on Postgres could
-- change after an UPDATE).
ALTER TABLE session_exercises ADD COLUMN IF NOT EXISTS position INTEGER NOT NULL DEFAULT 0;
ALTER TABLE exercise_sets ADD COLUMN IF NOT EXISTS position INTEGER NOT NULL DEFAULT 0;

-- Best-effort order for existing rows: session exercises in the order they were
-- added (then the workout's order for ties); sets by creation time, and planned
-- sets (which share one) with logged sets first, in the order they were logged.
UPDATE session_exercises SET position = ranked.pos
FROM (
    SELECT se.id, ROW_NUMBER() OVER (
        PARTITION BY se.session_id ORDER BY se.created_at, e.created_at, se.id
    ) - 1 AS pos
    FROM session_exercises se LEFT JOIN exercises e ON e.id = se.exercise_id
) ranked
WHERE session_exercises.id = ranked.id;

UPDATE exercise_sets SET position = ranked.pos
FROM (
    SELECT id, ROW_NUMBER() OVER (
        PARTITION BY session_exercise_id ORDER BY created_at, completed DESC, updated_at, id
    ) - 1 AS pos
    FROM exercise_sets
) ranked
WHERE exercise_sets.id = ranked.id;
