-- 0007_snapshot_source_url.sql — the exact page a snapshot was scraped from,
-- when the pipeline knows one. Nullable, and deliberately not backfilled for
-- existing rows (a full re-ingest to populate it retroactively is a real,
-- separate operation, not done here) — the web app falls back to each
-- source's homepage when this is NULL (web/pages/[weight].vue).
ALTER TABLE snapshots ADD COLUMN source_url TEXT;
