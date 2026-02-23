-- Create users table for authentication
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Index for case-insensitive email lookups
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users(LOWER(email));
