ALTER TABLE users
    ADD COLUMN IF NOT EXISTS user_type TEXT NOT NULL DEFAULT 'guest';

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email TEXT;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE users
SET user_type = CASE
    WHEN id LIKE 'usr_%' THEN 'auth'
    ELSE 'guest'
END
WHERE user_type IS NULL OR user_type NOT IN ('guest', 'auth');

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique_ci
    ON users (LOWER(email))
    WHERE email IS NOT NULL;
