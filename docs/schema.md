# Data Model — Rankings Snapshot Store
 
The store captures **weekly ranking snapshots** from multiple sources and
reconciles wrestlers across them. Movement is *derived* from comparing
snapshots, never stored raw.
 
DDL below is the live Postgres schema (`db/migrations/`) — see "Postgres
migration" at the end for how it got here and what changed from the original
SQLite version.
 
## Tables
 
```sql
-- Each ranking publisher, or a non-ranking data source (e.g. rosters).
CREATE TABLE sources (
  id    INTEGER PRIMARY KEY,
  name  TEXT NOT NULL UNIQUE,          -- 'FloWrestling', 'InterMat', 'WrestleStat', ...
  type  TEXT NOT NULL,                 -- 'editorial' | 'computer' | 'poll' | 'roster'
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

-- School-name canonicalization, keyed the same way wrestler_aliases is.
-- Introduced by the WrestleStat roster source (db/migrations/0002_roster.sql)
-- as a proper schools/school_aliases dimension, replacing the earlier
-- in-code canonicalSchools map that only knew FloWrestling's spellings.
CREATE TABLE school_aliases (
  id         INTEGER PRIMARY KEY,
  school_id  INTEGER NOT NULL REFERENCES schools(id),
  source_id  INTEGER NOT NULL REFERENCES sources(id),
  raw_name   TEXT NOT NULL,
  UNIQUE (source_id, raw_name)
);

-- A wrestler's roster listing for one school/season — not a ranked, dated
-- edition, so it doesn't fit snapshots/ranking_entries. Feeds the Fan Poll
-- ballot builder's wrestler pool.
CREATE TABLE roster_entries (
  id                INTEGER PRIMARY KEY,
  school_id         INTEGER NOT NULL REFERENCES schools(id),
  wrestler_id       INTEGER REFERENCES wrestlers(id),   -- NULLABLE until resolved
  season            INTEGER NOT NULL,                   -- ending year — the CURRENT roster season, generally AHEAD of snapshots.season; see §10
  weight_class      INTEGER,                            -- nullable only for genuine edge cases; see §8
  raw_name          TEXT NOT NULL,                       -- cleaned name used for matching
  raw_source_string TEXT NOT NULL,                       -- full published row text, verbatim
  captured_at       TEXT NOT NULL,
  UNIQUE (school_id, season, raw_name)
);

CREATE INDEX idx_roster_wrestler ON roster_entries(wrestler_id);
CREATE INDEX idx_roster_school   ON roster_entries(school_id, season);

-- Fan Poll user accounts. Auth (session cookies, password hashing) is
-- handled by nuxt-auth-utils in web/ — this table only stores what's needed
-- to verify a login. Web is this table's only writer (pipeline never touches
-- it), unlike every table above, which pipeline owns and web reads read-only.
CREATE TABLE users (
  id            INTEGER PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  display_name  TEXT,
  session_epoch INTEGER NOT NULL DEFAULT 1,  -- bump to invalidate every issued session at once
  created_at    TEXT NOT NULL
);

-- A user's rolling, never-locked top-33 at one weight class. No snapshot_id —
-- not a dated edition; the weekly aggregation job (Phase 4) takes the
-- point-in-time snapshot, so the ballot itself carries no draft/submitted/
-- locked state.
CREATE TABLE ballots (
  id           INTEGER PRIMARY KEY,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  weight_class INTEGER NOT NULL CHECK (weight_class IN (125,133,141,149,157,165,174,184,197,285)),
  season       INTEGER NOT NULL,          -- the current ROSTER season (§10), not the rankings display season
  updated_at   TEXT NOT NULL,
  UNIQUE (user_id, weight_class, season)
);

-- One ranked slot on a ballot. wrestler_id is a real FOREIGN KEY (everything
-- lives in one Postgres database — see "Postgres migration" below), unlike
-- the cross-file reference an earlier split-database design for this feature
-- would have needed.
CREATE TABLE ballot_entries (
  id          INTEGER PRIMARY KEY,
  ballot_id   INTEGER NOT NULL REFERENCES ballots(id) ON DELETE CASCADE,
  rank        INTEGER NOT NULL CHECK (rank BETWEEN 1 AND 33),
  wrestler_id INTEGER NOT NULL REFERENCES wrestlers(id),
  UNIQUE (ballot_id, rank),
  UNIQUE (ballot_id, wrestler_id)
);

CREATE INDEX idx_ballot_entries_wrestler ON ballot_entries(wrestler_id);
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
8. **Rosters are not rankings; `roster_entries` is a deliberately different
   shape than `ranking_entries`.** No `rank` column (a roster isn't ordered),
   no `snapshot_id` (not a dated edition — `season` is the only time
   dimension, since a roster is "current state," re-scraped and upserted, not
   accumulated week over week). `weight_class` is nullable, but not as a
   hedge against unreliable data: WrestleStat publishes a real, actively
   maintained weight per roster wrestler (docs/sources/wrestlestat.md);
   nullable only covers genuine edge cases (e.g. an unassigned walk-on). A
   wrestler can and does appear at a different weight in `roster_entries`
   (their team's current depth-chart assignment) than a ballot ranks them at
   (§4 of the Fan Poll implementation plan) — these are intentionally
   independent; the ballot's own weight governs where a ranking counts.
   `school_aliases` exists for the same reason `wrestler_aliases` does: a
   second source (WrestleStat) brings its own school-name spellings, now
   canonicalized as data instead of growing `resolve/schools.go`'s in-code
   map indefinitely.
9. **`session_epoch` is a deliberate substitute for a `sessions` table.**
   nuxt-auth-utils' session is a sealed, encrypted cookie with no server-side
   row to delete on logout/revocation — the only server-side lever is
   comparing the cookie's stamped epoch against the current column value on
   each request, so bumping it invalidates every outstanding session in one
   write. (No code bumps it yet — nothing forces re-auth today — but the
   column exists so a future feature that needs to can, without a migration.)
10. **`roster_entries.season` and `snapshots.season` are the same *kind* of
    value (ending year) but are NOT expected to be the same *number* at any
    given time, and that's normal, not a bug.** `snapshots.season` tracks
    whatever rankings have been published (currently 2026 — last season,
    ingested as launch backfill per `docs/decisions.md`; out-of-season means
    no live 2026-27 rankings yet). `roster_entries.season` tracks the
    *current* roster — as of this being written, 2027 (the upcoming 2026-27
    season, already underway roster-wise before it starts competing).
    `ballots.season` follows the roster season, not the rankings season: a
    ballot ranks wrestlers who are *currently on a roster*, which is a
    forward-looking pool independent of which rankings happen to be on
    display. Query the right one per table — never assume they match.
11. **A ballot is validated against `roster_entries`, not just against
    `wrestlers` existing.** `ballot_entries.wrestler_id` is a real FK to
    `wrestlers(id)`, which only guarantees the id is *some* canonical
    wrestler — including one who only ever appeared in an old, resolved
    ranking entry with no current roster listing. The write path
    (`server/api/ballots/[weight].patch.ts`) additionally checks every
    submitted id against the current season's `roster_entries` before
    accepting it, so a ballot can't reference someone who isn't part of the
    pool the picker actually shows.
## Postgres migration (2026-07-27)

The store is Postgres, not SQLite — migrated once the Fan Poll feature's open
public signup + concurrent user writes, and a move off the Pi to a hosted
PaaS, made SQLite's single-writer/single-machine model the wrong fit (see
`docs/decisions.md` Stack). The port from the original SQLite schema was
exactly the delta this section used to predict:
- `INTEGER PRIMARY KEY` → `INTEGER GENERATED ALWAYS AS IDENTITY`.
- Foreign keys enforced by default (no `PRAGMA foreign_keys` needed).
- Dates/timestamps **stayed** `TEXT` in ISO-8601, deliberately not switched to
  `DATE`/`TIMESTAMPTZ` — that would have rippled into every Go/TS call site's
  scanning code for no behavioral benefit at this scale.
- Everything else — including the LAG() movement query — is identical.
  `db/migrations/0001_init.sql` is the live DDL; this file remains its
  human-readable mirror.
 