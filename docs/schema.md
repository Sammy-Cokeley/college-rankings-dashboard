# Data Model — Rankings Snapshot Store
 
The store captures **weekly ranking snapshots** from multiple sources and
reconciles wrestlers across them. Movement is *derived* from comparing
snapshots, never stored raw.
 
DDL below is written portably (SQLite-leaning). Postgres deltas are noted at the
end. Pick the DB per `decisions.md`; the model is identical either way.
 
## Tables
 
```sql
-- Each ranking publisher.
CREATE TABLE sources (
  id    INTEGER PRIMARY KEY,
  name  TEXT NOT NULL UNIQUE,          -- 'FloWrestling', 'InterMat', 'NWCA Coaches'
  type  TEXT NOT NULL,                 -- 'editorial' | 'computer' | 'poll'
  url   TEXT
);
 
-- Canonical school / team.
CREATE TABLE schools (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL,           -- canonical, e.g. 'Iowa'
  conference  TEXT,
  division    TEXT NOT NULL DEFAULT 'DI'
);
 
-- Canonical wrestler identity.
CREATE TABLE wrestlers (
  id                INTEGER PRIMARY KEY,
  full_name         TEXT NOT NULL,
  current_school_id INTEGER REFERENCES schools(id),
  eligibility_year  TEXT               -- 'FR','SO','JR','SR','RS-FR' (informational)
);
 
-- Reconciliation layer: maps each source's raw strings -> one canonical wrestler.
CREATE TABLE wrestler_aliases (
  id          INTEGER PRIMARY KEY,
  wrestler_id INTEGER NOT NULL REFERENCES wrestlers(id),
  source_id   INTEGER NOT NULL REFERENCES sources(id),
  raw_name    TEXT NOT NULL,
  raw_school  TEXT,
  UNIQUE (source_id, raw_name, raw_school)
);
 
-- One published ranking list: a source, a weight, a date.
CREATE TABLE snapshots (
  id             INTEGER PRIMARY KEY,
  source_id      INTEGER NOT NULL REFERENCES sources(id),
  weight_class   INTEGER NOT NULL,     -- 125,133,141,149,157,165,174,184,197,285
  season         INTEGER NOT NULL,     -- ending year, e.g. 2026
  published_date DATE NOT NULL,        -- canonical time spine
  captured_at    TIMESTAMP NOT NULL,   -- when we scraped it
  UNIQUE (source_id, weight_class, season, published_date)
);
 
-- A wrestler's position within one snapshot.
CREATE TABLE ranking_entries (
  id                INTEGER PRIMARY KEY,
  snapshot_id       INTEGER NOT NULL REFERENCES snapshots(id),
  wrestler_id       INTEGER REFERENCES wrestlers(id),  -- NULLABLE until resolved
  rank              INTEGER NOT NULL,
  raw_source_string TEXT NOT NULL,     -- published identity string; never lose this
  raw_school        TEXT,              -- school as published this week (point-in-time)
  raw_grade         TEXT,              -- eligibility as published: FR/SO/JR/SR
  UNIQUE (snapshot_id, rank, raw_source_string)  -- allows a genuine tie; see §7
);
 
CREATE INDEX idx_entries_wrestler ON ranking_entries(wrestler_id);
CREATE INDEX idx_aliases_lookup   ON wrestler_aliases(source_id, raw_name);
```
 
## Design decisions (and why)
 
1. **`raw_source_string` on every entry, always.** Ingestion captures exactly
   what the source published before any matching. This makes resolution
   auditable and lets you reprocess matches later without re-scraping. For
   sources that split fields (FloWrestling gives Name / School / Grade as
   separate cells), `raw_source_string` holds the published **identity string**
   (the name), and the rest is captured in `raw_school` / `raw_grade`.

   `raw_school` / `raw_grade` are **point-in-time, on the entry — not the
   wrestler.** A wrestler's school and eligibility *as a given week's ranking
   published them* belong to that snapshot: wrestlers transfer mid-season and
   grade advances each year. So these are stored verbatim per entry (display +
   stats read them directly), while `wrestlers.current_school_id` /
   `eligibility_year` remain "latest known," derived from the most recent
   entries. Both are nullable — a future source that emits only a combined name
   string would leave them NULL.
2. **`wrestler_id` is nullable.** Ingest first, resolve second. A scrape never
   fails because a name didn't match; unresolved entries sit with a null
   `wrestler_id` and get reconciled in a later pass.
3. **Weight class lives on the snapshot, not the wrestler.** A wrestler ranked
   at 149 by one source and 157 by another — or who moves weights mid-season —
   is represented naturally as different snapshots/entries. `wrestlers` has no
   weight column; "current weight" is just whatever their latest entries say.
4. **`published_date` is the time spine; derive week numbers in the app.**
   Sources publish on different schedules, so there is no shared "week 7." Anchor
   on dates, align by nearest, compute display week numbers downstream.
5. **Movement is derived, never stored.** A wrestler's movement at a weight for a
   source = compare their `rank` across consecutive snapshots ordered by
   `published_date`. Expose as a query or view, not a column. Example shape:
   ```sql
   -- previous rank for each entry, same source+weight+season+wrestler
   SELECT e.*, s.published_date,
          LAG(e.rank) OVER (
            PARTITION BY s.source_id, s.weight_class, s.season, e.wrestler_id
            ORDER BY s.published_date
          ) AS prev_rank
   FROM ranking_entries e
   JOIN snapshots s ON s.id = e.snapshot_id;
   ```
 
6. **No consensus/composite table.** A blended cross-source ranking is
   deliberately out of the model for now (display-first; sources presented
   individually and attributed). If/when added, it's a derived view, not a
   stored authority.
7. **Rank is not unique within a snapshot; ties are stored verbatim.** The entry
   key is `UNIQUE (snapshot_id, rank, raw_source_string)`, not
   `(snapshot_id, rank)`. Flo's hand-entered tables do publish two wrestlers at
   the same rank (confirmed 197, 2026-03-27 — likely an editorial typo, but we
   can't distinguish that from an intentional tie, and either way the rule is to
   store what the source published, never a silently renumbered version). A
   stricter constraint would reject the second row and — because an edition
   ingests in one transaction — discard the *entire* snapshot over one tie,
   violating "never block ingestion / never lose raw." Including
   `raw_source_string` in the key still rejects the realistic bug: the same row
   inserted twice. Consumers must treat rank as non-unique (order by
   `rank, raw_source_string`); movement (§5) is unaffected, as it partitions by
   wrestler and orders by `published_date`, not by rank.
## Postgres deltas
 
If you choose Postgres instead of SQLite:
- `INTEGER PRIMARY KEY` → `INTEGER GENERATED ALWAYS AS IDENTITY` (or `BIGSERIAL`).
- `TIMESTAMP` → `TIMESTAMPTZ`.
- `DATE` is the same.
- Everything else is identical. The model is simple enough to port either
  direction in an afternoon, so SQLite-now does not lock you in.
 