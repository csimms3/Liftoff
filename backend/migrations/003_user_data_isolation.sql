-- Add user_id to workouts for data isolation
ALTER TABLE workouts ADD COLUMN IF NOT EXISTS user_id VARCHAR(36) REFERENCES users(id);

-- Add user_id to workout_sessions
ALTER TABLE workout_sessions ADD COLUMN IF NOT EXISTS user_id VARCHAR(36) REFERENCES users(id);

-- Add user_id to dino_game_scores
ALTER TABLE dino_game_scores ADD COLUMN IF NOT EXISTS user_id VARCHAR(36) REFERENCES users(id);

-- Owner for rows that predate user accounts. Only created when such rows exist,
-- and its password hash is not a valid bcrypt hash, so nobody can log in as it.
INSERT INTO users (id, email, password_hash, created_at)
SELECT '00000000-0000-0000-0000-000000000001', 'admin@liftoff.local', '!locked', NOW()
WHERE NOT EXISTS (SELECT 1 FROM users WHERE id = '00000000-0000-0000-0000-000000000001')
  AND (EXISTS (SELECT 1 FROM workouts WHERE user_id IS NULL)
    OR EXISTS (SELECT 1 FROM workout_sessions WHERE user_id IS NULL)
    OR EXISTS (SELECT 1 FROM dino_game_scores WHERE user_id IS NULL));

-- Migrate existing workouts to admin user
UPDATE workouts SET user_id = '00000000-0000-0000-0000-000000000001' WHERE user_id IS NULL;

-- Migrate existing workout_sessions to admin user  
UPDATE workout_sessions SET user_id = '00000000-0000-0000-0000-000000000001' WHERE user_id IS NULL;

-- Migrate existing dino_game_scores to admin user
UPDATE dino_game_scores SET user_id = '00000000-0000-0000-0000-000000000001' WHERE user_id IS NULL;

-- Make user_id NOT NULL
ALTER TABLE workouts ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE workout_sessions ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE dino_game_scores ALTER COLUMN user_id SET NOT NULL;

-- Add indexes for user lookups
CREATE INDEX IF NOT EXISTS idx_workouts_user_id ON workouts(user_id);
CREATE INDEX IF NOT EXISTS idx_workout_sessions_user_id ON workout_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_dino_game_scores_user_id ON dino_game_scores(user_id);
