-- Admin is a per-user flag, set by hand:
--   UPDATE users SET is_admin = true WHERE email = 'you@example.com';
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;

-- Lock the account that older versions seeded with a public password (Admin123!).
-- The row and any data it owns are kept; only login and admin rights are removed.
UPDATE users SET password_hash = '!locked', is_admin = false
WHERE id = '00000000-0000-0000-0000-000000000001';
DELETE FROM password_reset_tokens WHERE user_id = '00000000-0000-0000-0000-000000000001';
