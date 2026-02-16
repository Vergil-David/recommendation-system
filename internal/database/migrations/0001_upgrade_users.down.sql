DROP TRIGGER IF EXISTS users_set_updated_at ON users;
DROP FUNCTION IF EXISTS set_updated_at();

DROP INDEX IF EXISTS users_email_uq;
DROP INDEX IF EXISTS users_username_uq;

ALTER TABLE users
    DROP COLUMN IF EXISTS role,
    DROP COLUMN IF EXISTS email_verified,
    DROP COLUMN IF EXISTS is_active,
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS display_name,
    DROP COLUMN IF EXISTS avatar_url,
    DROP COLUMN IF EXISTS bio;

ALTER TABLE users
    ALTER COLUMN email TYPE text USING email::text,
    ALTER COLUMN username TYPE text USING username::text;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
        DROP TYPE user_role;
    END IF;
END
$$;
