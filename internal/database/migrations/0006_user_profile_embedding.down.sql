DROP INDEX IF EXISTS users_profile_embedding_idx;

ALTER TABLE public.users
    DROP COLUMN IF EXISTS profile_embedding;
