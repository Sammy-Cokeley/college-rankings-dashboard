-- 0003_users.sql — Fan Poll user accounts (Phase 2 of the community-ballots
-- feature; docs/decisions.md "v1 scope" — poll approved as a v1 extension).
-- Auth itself is handled by nuxt-auth-utils (sealed, encrypted cookie
-- sessions) — this table only stores what's needed to verify a login and
-- recognize a returning user.
--
-- session_epoch replaces a separate sessions table: nuxt-auth-utils' cookie
-- IS the session (encrypted client-side, no server-side session store), so
-- there's nothing to revoke server-side except by changing what the cookie's
-- checked against on each request. Bumping session_epoch invalidates every
-- previously issued session in one write (password change, moderation)
-- without needing a sessions table to delete rows from.
--
-- created_at is TEXT (ISO-8601), not TIMESTAMPTZ — matches every other
-- timestamp column in this schema (see 0001_init.sql's Postgres-migration
-- header note: kept TEXT deliberately, no native-date-type benefit at this
-- scale).
CREATE TABLE users (
  id            INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE,       -- lowercased at the app boundary before insert
  password_hash TEXT NOT NULL,              -- scrypt, via nuxt-auth-utils hashPassword
  display_name  TEXT,                       -- optional; UI falls back to the email's local part
  session_epoch INTEGER NOT NULL DEFAULT 1,
  created_at    TEXT NOT NULL
);
