-- 0001_init.sql — initial schema (Postgres)
--
-- Ported from the original SQLite schema (docs/decisions.md "Postgres
-- migration" — the platform moved off the Pi once open public signup +
-- concurrent user writes made SQLite's single-writer model the wrong fit).
--
-- Postgres specifics:
--   * `GENERATED ALWAYS AS IDENTITY` replaces SQLite's implicit rowid-alias
--     `INTEGER PRIMARY KEY` auto-increment.
--   * Dates/timestamps stay TEXT in ISO-8601, same as the SQLite original
--     ('2026-01-13', '2026-01-13T18:04:00Z') — they sort correctly as text and
--     the LAG() movement query (schema.md §5) doesn't need a native date type.
--     Deliberately not switched to DATE/TIMESTAMPTZ: it would ripple into every
--     Go/TS call site's scanning code for no behavioral benefit at this scale.
--   * Foreign keys are enforced by default (no PRAGMA needed, unlike SQLite).

CREATE TABLE sources (
  id    INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name  TEXT NOT NULL UNIQUE,                 -- 'FloWrestling', 'InterMat', ...
  type  TEXT NOT NULL CHECK (type IN ('editorial','computer','poll')),
  url   TEXT
);

CREATE TABLE schools (
  id          INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name        TEXT NOT NULL,                  -- canonical, e.g. 'Iowa'
  conference  TEXT,
  division    TEXT NOT NULL DEFAULT 'DI'
);

CREATE TABLE wrestlers (
  id                INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  full_name         TEXT NOT NULL,
  current_school_id INTEGER REFERENCES schools(id),
  eligibility_year  TEXT                       -- 'FR','SO','JR','SR','RS-FR' (info only)
);

-- Reconciliation layer: each source's raw strings -> one canonical wrestler.
CREATE TABLE wrestler_aliases (
  id          INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  wrestler_id INTEGER NOT NULL REFERENCES wrestlers(id),
  source_id   INTEGER NOT NULL REFERENCES sources(id),
  raw_name    TEXT NOT NULL,
  raw_school  TEXT,
  UNIQUE (source_id, raw_name, raw_school)
);

-- One published ranking list: a source, a weight, a date.
CREATE TABLE snapshots (
  id             INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  source_id      INTEGER NOT NULL REFERENCES sources(id),
  weight_class   INTEGER NOT NULL,            -- 125,133,141,149,157,165,174,184,197,285
  season         INTEGER NOT NULL,            -- ending year, e.g. 2026
  published_date TEXT NOT NULL,               -- ISO-8601 date, canonical time spine
  captured_at    TEXT NOT NULL,               -- ISO-8601 datetime, when scraped
  UNIQUE (source_id, weight_class, season, published_date)
);

-- A wrestler's position within one snapshot.
CREATE TABLE ranking_entries (
  id                INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  snapshot_id       INTEGER NOT NULL REFERENCES snapshots(id),
  wrestler_id       INTEGER REFERENCES wrestlers(id),   -- NULLABLE until resolved
  rank              INTEGER NOT NULL,
  raw_source_string TEXT NOT NULL,                      -- published identity string (e.g. the name cell)
  raw_school        TEXT,                               -- school as published this week (point-in-time)
  raw_grade         TEXT,                               -- eligibility as published (FR/SO/JR/SR)
  -- raw_source_string is part of the key so a snapshot can hold a genuine tie
  -- (two wrestlers published at the same rank — observed in Flo's hand-entered
  -- tables, e.g. 197 2026-03-27). It is NOT UNIQUE(snapshot_id, rank): storing
  -- the source's published rank verbatim beats silently renumbering it, and one
  -- tie must never reject the whole edition. The key still catches the realistic
  -- bug — the exact same row inserted twice.
  UNIQUE (snapshot_id, rank, raw_source_string)
);

CREATE INDEX idx_entries_wrestler ON ranking_entries(wrestler_id);
CREATE INDEX idx_aliases_lookup   ON wrestler_aliases(source_id, raw_name);
