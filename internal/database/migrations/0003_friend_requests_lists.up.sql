CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE public.friendships
    ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

UPDATE public.friendships
SET created_at = now()
WHERE created_at IS NULL;

UPDATE public.friendships
SET updated_at = coalesce(created_at, now())
WHERE updated_at IS NULL;

ALTER TABLE public.friendships
    ALTER COLUMN created_at SET DEFAULT now(),
    ALTER COLUMN created_at SET NOT NULL,
    ALTER COLUMN updated_at SET DEFAULT now(),
    ALTER COLUMN updated_at SET NOT NULL;

CREATE OR REPLACE FUNCTION set_friendships_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS friendships_set_updated_at ON public.friendships;
CREATE TRIGGER friendships_set_updated_at
BEFORE UPDATE ON public.friendships
FOR EACH ROW
EXECUTE FUNCTION set_friendships_updated_at();

CREATE INDEX IF NOT EXISTS friendships_user_status_idx
    ON public.friendships (user_id, status);

CREATE INDEX IF NOT EXISTS friendships_friend_status_idx
    ON public.friendships (friend_id, status);

CREATE INDEX IF NOT EXISTS friendships_user_friend_idx
    ON public.friendships (user_id, friend_id);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'username'
    ) THEN
        EXECUTE 'CREATE INDEX IF NOT EXISTS users_username_search_trgm_idx ON public.users USING gin (lower(username::text) gin_trgm_ops)';
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'email'
    ) THEN
        EXECUTE 'CREATE INDEX IF NOT EXISTS users_email_search_trgm_idx ON public.users USING gin (lower(email::text) gin_trgm_ops)';
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'display_name'
    ) THEN
        EXECUTE 'CREATE INDEX IF NOT EXISTS users_display_name_search_trgm_idx ON public.users USING gin (lower(coalesce(display_name, '''')) gin_trgm_ops)';
    END IF;
END
$$;
