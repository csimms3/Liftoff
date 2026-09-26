-- Routines: named, ordered groups of workouts
CREATE TABLE IF NOT EXISTS routines (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_routines_user_id ON routines(user_id);

CREATE TABLE IF NOT EXISTS routine_workouts (
    id VARCHAR(36) PRIMARY KEY,
    routine_id VARCHAR(36) NOT NULL REFERENCES routines(id) ON DELETE CASCADE,
    workout_id VARCHAR(36) NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    slot_order INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_routine_workouts_routine_id ON routine_workouts(routine_id);
