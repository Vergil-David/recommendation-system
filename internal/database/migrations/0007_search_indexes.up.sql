CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS items_title_trgm_idx
    ON items USING gin (title gin_trgm_ops);

CREATE INDEX IF NOT EXISTS items_description_trgm_idx
    ON items USING gin (description gin_trgm_ops);
