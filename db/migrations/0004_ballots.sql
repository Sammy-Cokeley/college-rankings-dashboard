-- 0004_ballots.sql — Fan Poll ballots (Phase 3 of the community-ballots
-- feature). A ballot is a user's rolling, never-locked top-33 at one weight
-- class — not a dated edition (no snapshot_id): the weekly aggregation job
-- (Phase 4, not yet built) is what takes the point-in-time snapshot, so the
-- ballot itself needs no draft/submitted/locked state machine.
--
-- wrestler_id is a real FOREIGN KEY (unlike the split-database design
-- considered earlier in this feature's planning — superseded once everything
-- moved into one Postgres database, see docs/decisions.md "Postgres
-- migration"): ballot_entries.wrestler_id -> wrestlers(id) is enforced by
-- Postgres directly, no application-level integrity gap.
CREATE TABLE ballots (
  id           INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  weight_class INTEGER NOT NULL CHECK (weight_class IN (125,133,141,149,157,165,174,184,197,285)),
  season       INTEGER NOT NULL,          -- ending year; the CURRENT roster season, not the rankings display season (schema.md §10)
  updated_at   TEXT NOT NULL,
  UNIQUE (user_id, weight_class, season)  -- one ballot per user per weight per season
);

-- A ballot slot: 1..33, at most one wrestler per rank, no wrestler twice on
-- the same ballot. The wrestler's weight here is whatever the BALLOT's own
-- weight_class is — independent of that wrestler's roster-assigned weight
-- (schema.md §8): a user can deliberately rank someone outside their
-- WrestleStat-listed weight.
CREATE TABLE ballot_entries (
  id          INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ballot_id   INTEGER NOT NULL REFERENCES ballots(id) ON DELETE CASCADE,
  rank        INTEGER NOT NULL CHECK (rank BETWEEN 1 AND 33),
  wrestler_id INTEGER NOT NULL REFERENCES wrestlers(id),
  UNIQUE (ballot_id, rank),
  UNIQUE (ballot_id, wrestler_id)
);

CREATE INDEX idx_ballot_entries_wrestler ON ballot_entries(wrestler_id);
