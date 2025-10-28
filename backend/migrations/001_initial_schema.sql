-- Create workouts table
CREATE TABLE IF NOT EXISTS workouts (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create exercises table
CREATE TABLE IF NOT EXISTS exercises (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    sets INTEGER NOT NULL,
    reps INTEGER NOT NULL,
    weight DECIMAL(8,2) NOT NULL DEFAULT 0,
    workout_id VARCHAR(36) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (workout_id) REFERENCES workouts(id) ON DELETE CASCADE
);

-- Create workout_sessions table
CREATE TABLE IF NOT EXISTS workout_sessions (
    id VARCHAR(36) PRIMARY KEY,
    workout_id VARCHAR(36) NOT NULL,
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMP NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (workout_id) REFERENCES workouts(id) ON DELETE CASCADE
);

-- Create session_exercises table
CREATE TABLE IF NOT EXISTS session_exercises (
    id VARCHAR(36) PRIMARY KEY,
    session_id VARCHAR(36) NOT NULL,
    exercise_id VARCHAR(36) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (session_id) REFERENCES workout_sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (exercise_id) REFERENCES exercises(id) ON DELETE CASCADE
);

-- Create exercise_sets table
CREATE TABLE IF NOT EXISTS exercise_sets (
    id VARCHAR(36) PRIMARY KEY,
    session_exercise_id VARCHAR(36) NOT NULL,
    reps INTEGER NOT NULL,
    weight DECIMAL(8,2) NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT false,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (session_exercise_id) REFERENCES session_exercises(id) ON DELETE CASCADE
);

-- Create dino_game_scores table
CREATE TABLE IF NOT EXISTS dino_game_scores (
    id VARCHAR(36) PRIMARY KEY,
    score INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_exercises_workout_id ON exercises(workout_id);
CREATE INDEX IF NOT EXISTS idx_workout_sessions_workout_id ON workout_sessions(workout_id);
CREATE INDEX IF NOT EXISTS idx_workout_sessions_is_active ON workout_sessions(is_active);
CREATE INDEX IF NOT EXISTS idx_session_exercises_session_id ON session_exercises(session_id);
CREATE INDEX IF NOT EXISTS idx_exercise_sets_session_exercise_id ON exercise_sets(session_exercise_id);
