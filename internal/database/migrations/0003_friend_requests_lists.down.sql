DROP INDEX IF EXISTS public.users_display_name_search_trgm_idx;
DROP INDEX IF EXISTS public.users_email_search_trgm_idx;
DROP INDEX IF EXISTS public.users_username_search_trgm_idx;

DROP INDEX IF EXISTS public.friendships_user_friend_idx;

DROP TRIGGER IF EXISTS friendships_set_updated_at ON public.friendships;
DROP FUNCTION IF EXISTS set_friendships_updated_at();
