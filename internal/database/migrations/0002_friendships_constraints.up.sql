DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'friendships_status_check'
          AND conrelid = 'public.friendships'::regclass
    ) THEN
        ALTER TABLE public.friendships
            ADD CONSTRAINT friendships_status_check
            CHECK (status IN ('pending', 'accepted', 'rejected'));
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'friendships_no_self_check'
          AND conrelid = 'public.friendships'::regclass
    ) THEN
        ALTER TABLE public.friendships
            ADD CONSTRAINT friendships_no_self_check
            CHECK (user_id <> friend_id);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'friendships_user_friend_unique'
          AND conrelid = 'public.friendships'::regclass
    ) THEN
        ALTER TABLE public.friendships
            ADD CONSTRAINT friendships_user_friend_unique UNIQUE (user_id, friend_id);
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS friendships_friend_status_idx
    ON public.friendships (friend_id, status);

CREATE INDEX IF NOT EXISTS friendships_user_status_idx
    ON public.friendships (user_id, status);
