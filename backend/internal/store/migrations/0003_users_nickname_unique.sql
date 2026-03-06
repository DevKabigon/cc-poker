WITH duplicated AS (
    SELECT id
    FROM (
        SELECT
            id,
            ROW_NUMBER() OVER (
                PARTITION BY LOWER(nickname)
                ORDER BY created_at, id
            ) AS row_num
        FROM users
    ) ranked
    WHERE row_num > 1
)
UPDATE users AS u
SET nickname = CONCAT('user_', SUBSTRING(MD5(u.id) FROM 1 FOR 8))
FROM duplicated AS d
WHERE u.id = d.id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_nickname_unique_ci
    ON users (LOWER(nickname));

