-- Admin is a per-user flag, set by hand:
--   UPDATE users SET is_admin = true WHERE email = 'you@example.com';
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;

-- Lock the account that older versions seeded with a public password (Admin123!).
-- Match by email too: a user who registered admin@liftoff.local before the seed
-- existed had that password forced onto their row on every boot.
-- The rows and any data they own are kept; only login and admin rights are removed.
DELETE FROM password_reset_tokens WHERE user_id IN (
    SELECT id FROM users
    WHERE id = '00000000-0000-0000-0000-000000000001' OR LOWER(email) = 'admin@liftoff.local'
);
UPDATE users SET password_hash = '!locked', is_admin = false
WHERE id = '00000000-0000-0000-0000-000000000001' OR LOWER(email) = 'admin@liftoff.local';
