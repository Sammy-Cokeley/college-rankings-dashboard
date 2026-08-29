-- Seed: the Fan Poll source. Idempotent (ON CONFLICT DO NOTHING), same
-- pattern as 0001_sources.sql / 0002_wrestlestat_source.sql. type='poll' was
-- reserved in the schema from the start (schema.md design decision #6) —
-- this is the first thing to actually use it. Written only by
-- pipeline/cmd/aggregate-poll, from aggregated ballots — never a stored
-- composite of other sources (schema.md #6/#8).
INSERT INTO sources (name, type, url) VALUES ('Fan Poll', 'poll', NULL)
  ON CONFLICT (name) DO NOTHING;
