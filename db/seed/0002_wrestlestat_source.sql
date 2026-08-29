-- Seed: the WrestleStat roster source. Idempotent (ON CONFLICT DO NOTHING),
-- same pattern as 0001_sources.sql.
INSERT INTO sources (name, type, url) VALUES ('WrestleStat', 'roster', 'https://www.wrestlestat.com')
  ON CONFLICT (name) DO NOTHING;
