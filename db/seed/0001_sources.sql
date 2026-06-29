-- Seed: the v0 ranking sources.
--
-- Idempotent by design: sources.name is UNIQUE and INSERT OR IGNORE makes
-- re-running ApplySeeds a no-op. FloWrestling is v0's only editorial source.
INSERT OR IGNORE INTO sources (name, type, url) VALUES ('FloWrestling', 'editorial', NULL);
