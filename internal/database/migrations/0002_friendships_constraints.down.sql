DROP INDEX IF EXISTS public.friendships_friend_status_idx;
DROP INDEX IF EXISTS public.friendships_user_status_idx;

ALTER TABLE public.friendships
    DROP CONSTRAINT IF EXISTS friendships_user_friend_unique;

DROP INDEX IF EXISTS public.friendships_user_friend_unique_idx;

ALTER TABLE public.friendships
    DROP CONSTRAINT IF EXISTS friendships_no_self_check;

ALTER TABLE public.friendships
    DROP CONSTRAINT IF EXISTS friendships_status_check;
