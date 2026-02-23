-- Add user_id to workouts for data isolation
ALTER TABLE workouts ADD COLUMN IF NOT EXISTS user_id VARCHAR(36) REFERENCES users(id);

-- Add user_id to workout_sessions
ALTER TABLE workout_sessions ADD COLUMN IF NOT EXISTS user_id VARCHAR(36) REFERENCES users(id);

-- Add user_id to dino_game_scores
ALTER TABLE dino_game_scores ADD COLUMN IF NOT EXISTS user_id VARCHAR(36) REFERENCES users(id);

-- Create default admin user for migrated data (password: Admin123! - change after first login)
-- Run this only if migrating existing data. For fresh installs, this user may be created by app.
INSERT INTO users (id, email, password_hash, created_at)
SELECT 
    '00000000-0000-0000-0000-000000000001',
    'admin@liftoff.local',
    '$2a$10$SlmOtj3A17j2JLju8e9VfeHZo/SjwuC4ciN0mbSXR9gILDiuaJexe',
    NOW()
WHERE NOT EXISTS (SELECT 1 FROM users WHERE email = 'admin@liftoff.local');

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
