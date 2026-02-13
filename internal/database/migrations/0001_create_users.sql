-- Extensions (idempotent)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "citext";

-- Optional: enum for role
DO
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
    CREATE TYPE user_role AS ENUM ('user', 'admin');
  END IF;
END;

CREATE TABLE IF NOT EXISTS users (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),

  email           citext NOT NULL,
  username        citext NOT NULL,

  password_hash   text NOT NULL,

  display_name    text,
  avatar_url      text,
  bio             text,

  role            user_role NOT NULL DEFAULT 'user',

  email_verified  boolean NOT NULL DEFAULT false,
  is_active       boolean NOT NULL DEFAULT true,

  last_login_at   timestamptz,

  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),

  -- Basic sanity checks (validate more strictly in app code)
  CONSTRAINT users_email_min_len CHECK (char_length(email) >= 5),
  CONSTRAINT users_username_min_len CHECK (char_length(username) >= 3),
  CONSTRAINT users_password_hash_min_len CHECK (char_length(password_hash) >= 20)
);

-- Uniqueness
CREATE UNIQUE INDEX IF NOT EXISTS users_email_uq ON users (email);
CREATE UNIQUE INDEX IF NOT EXISTS users_username_uq ON users (username);

-- Helpful indexes
CREATE INDEX IF NOT EXISTS users_created_at_idx ON users (created_at);

-- updated_at trigger
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS users_set_updated_at ON users;
CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
