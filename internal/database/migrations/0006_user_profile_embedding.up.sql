-- Add profile_embedding column to users table.
-- This stores a weighted average of liked/favorite item embeddings
-- so that recommendations can use a single pgvector query instead of O(N).
ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS profile_embedding vector(384);

-- Index for efficient nearest-neighbor search against profile vectors.
CREATE INDEX IF NOT EXISTS users_profile_embedding_idx
    ON public.users
    USING ivfflat (profile_embedding vector_l2_ops)
    WITH (lists = 10);
