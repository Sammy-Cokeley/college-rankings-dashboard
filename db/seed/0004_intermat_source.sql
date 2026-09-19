-- Seed: the InterMat source. Idempotent (ON CONFLICT DO NOTHING), same
-- pattern as 0001_sources.sql / 0002_wrestlestat_source.sql. type='editorial'
-- like FloWrestling — a human-compiled rankings list, not a computer ranking.
INSERT INTO sources (name, type, url) VALUES ('InterMat', 'editorial', 'https://intermatwrestle.com')
  ON CONFLICT (name) DO NOTHING;
