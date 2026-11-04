CREATE TABLE IF NOT EXISTS public.interactions (
    id serial PRIMARY KEY,
    user_id uuid REFERENCES public.users(id) ON DELETE CASCADE,
    item_id integer REFERENCES public.items(id) ON DELETE CASCADE,
    rating integer,
    is_liked boolean NOT NULL DEFAULT false,
    viewed_at timestamptz DEFAULT now(),
    interaction_type text NOT NULL DEFAULT 'view',
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT interactions_user_id_item_id_key UNIQUE (user_id, item_id)
);

ALTER TABLE public.interactions
    ADD COLUMN IF NOT EXISTS interaction_type text;

ALTER TABLE public.interactions
    ADD COLUMN IF NOT EXISTS updated_at timestamptz;

UPDATE public.interactions
SET interaction_type = CASE
    WHEN interaction_type IS NOT NULL THEN interaction_type
    WHEN COALESCE(is_liked, false) = true THEN 'like'
    WHEN viewed_at IS NOT NULL THEN 'view'
    ELSE 'view'
END
WHERE interaction_type IS NULL;

UPDATE public.interactions
SET updated_at = COALESCE(updated_at, viewed_at, now())
WHERE updated_at IS NULL;

ALTER TABLE public.interactions
    ALTER COLUMN interaction_type SET DEFAULT 'view',
    ALTER COLUMN updated_at SET DEFAULT now();

-- NOTE: The interaction_type constraint is created by migration 0005
-- with the correct values ('viewed', 'liked', 'disliked', 'favorite', 'skipped').
-- We intentionally do NOT create a constraint here to avoid inconsistency.

CREATE INDEX IF NOT EXISTS interactions_user_id_idx
    ON public.interactions (user_id);

CREATE INDEX IF NOT EXISTS interactions_user_id_updated_at_idx
    ON public.interactions (user_id, updated_at DESC);
