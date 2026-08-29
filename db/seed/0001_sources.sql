-- Seed: the v0 ranking sources.
--
-- Idempotent by design: sources.name is UNIQUE and ON CONFLICT DO NOTHING
-- makes re-running ApplySeeds a no-op. FloWrestling is v0's only editorial
-- source. (Postgres: ON CONFLICT DO NOTHING replaces SQLite's INSERT OR IGNORE.)
INSERT INTO sources (name, type, url) VALUES ('FloWrestling', 'editorial', NULL)
  ON CONFLICT (name) DO NOTHING;
