-- Explicit exercise order within a workout, so a session's reordering can be
-- saved back to it (the Finish step). Existing order (creation time) is kept.
ALTER TABLE exercises ADD COLUMN IF NOT EXISTS position INTEGER NOT NULL DEFAULT 0;

UPDATE exercises e SET position = r.n
FROM (SELECT id, ROW_NUMBER() OVER (PARTITION BY workout_id ORDER BY created_at, id) - 1 AS n FROM exercises) r
WHERE e.id = r.id;
