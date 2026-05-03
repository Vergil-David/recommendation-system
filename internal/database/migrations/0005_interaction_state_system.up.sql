ALTER TABLE public.interactions
    ADD COLUMN IF NOT EXISTS viewed boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS liked boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS disliked boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS skipped boolean NOT NULL DEFAULT false;

UPDATE public.interactions
SET interaction_type = CASE interaction_type
    WHEN 'view' THEN 'viewed'
    WHEN 'like' THEN 'liked'
    WHEN 'skip' THEN 'skipped'
    ELSE interaction_type
END
WHERE interaction_type IN ('view', 'like', 'skip');

UPDATE public.interactions
SET
    viewed = viewed OR interaction_type = 'viewed',
    liked = liked OR interaction_type = 'liked' OR COALESCE(is_liked, false),
    disliked = disliked OR interaction_type = 'disliked',
    favorite = favorite OR interaction_type = 'favorite',
    skipped = skipped OR interaction_type = 'skipped';

UPDATE public.interactions
SET is_liked = liked
WHERE is_liked IS DISTINCT FROM liked;

ALTER TABLE public.interactions
    ALTER COLUMN interaction_type SET DEFAULT 'viewed';

ALTER TABLE public.interactions
    DROP CONSTRAINT IF EXISTS interactions_interaction_type_check;

ALTER TABLE public.interactions
    ADD CONSTRAINT interactions_interaction_type_check
    CHECK (interaction_type IN ('viewed', 'liked', 'disliked', 'favorite', 'skipped'));
