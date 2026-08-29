-- 0002_roster.sql — WrestleStat roster pull: the wrestler pool for the Fan
-- Poll ballot builder, and the first real populator of `schools` (a curated
-- schools/school_aliases dimension was already anticipated by
-- pipeline/internal/resolve/schools.go's own comment on adding a second
-- source; also unblocks the deferred conference-filter bump chart cut noted
-- in docs/decisions.md).
--
-- Not shaped like snapshots/ranking_entries: a roster listing is not a
-- ranked, dated edition — it's "who's on this team's roster, at what weight,
-- this season." weight_class is nullable only for genuine edge cases
-- (WrestleStat's own weight field is real, actively-maintained data — see
-- docs/sources/wrestlestat.md — not a risk-hedged fallback for unreliable
-- data).

CREATE TABLE school_aliases (
  id         INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  school_id  INTEGER NOT NULL REFERENCES schools(id),
  source_id  INTEGER NOT NULL REFERENCES sources(id),
  raw_name   TEXT NOT NULL,
  UNIQUE (source_id, raw_name)
);

CREATE TABLE roster_entries (
  id                INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  school_id         INTEGER NOT NULL REFERENCES schools(id),
  wrestler_id       INTEGER REFERENCES wrestlers(id),   -- NULLABLE until resolved
  season            INTEGER NOT NULL,                   -- ending year, matches snapshots.season
  weight_class      INTEGER,                            -- nullable only for genuine edge cases; see header
  raw_name          TEXT NOT NULL,                       -- cleaned name used for matching (resolve.go)
  raw_source_string TEXT NOT NULL,                       -- full published row text, verbatim, never lost
  captured_at       TEXT NOT NULL,                       -- ISO-8601 datetime, when scraped
  UNIQUE (school_id, season, raw_name)                    -- idempotent re-scrape guard
);

CREATE INDEX idx_roster_wrestler ON roster_entries(wrestler_id);
CREATE INDEX idx_roster_school   ON roster_entries(school_id, season);

-- sources.type needs a 'roster' value: a roster pull isn't a ranking source
-- in the editorial/computer/poll sense, but reuses the same sources
-- dimension for provenance rather than a parallel one-purpose table.
ALTER TABLE sources DROP CONSTRAINT sources_type_check;
ALTER TABLE sources ADD CONSTRAINT sources_type_check
  CHECK (type IN ('editorial', 'computer', 'poll', 'roster'));
