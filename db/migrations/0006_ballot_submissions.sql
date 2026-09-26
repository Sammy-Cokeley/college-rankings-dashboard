-- 0006_ballot_submissions.sql — a user's ballot submission history (the
-- "Phase 4" the 0004_ballots.sql header comment anticipated: something takes
-- a point-in-time snapshot so the ballot itself can stay rolling/never-
-- locked). Unlike ballots/ballot_entries, this is append-only: one new row
-- pair per Submit click, never updated or replaced — the history IS the
-- table, no separate "current" pointer needed. The weekly aggregation job
-- (cmd/aggregate-poll) reads each user's MOST RECENT submission as of its
-- run time, which gets carry-over (an un-resubmitted ballot still counts)
-- for free, with no extra "which week" bookkeeping in the schema.
CREATE TABLE ballot_submissions (
  id           INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  weight_class INTEGER NOT NULL CHECK (weight_class IN (125,133,141,149,157,165,174,184,197,285)),
  season       INTEGER NOT NULL,          -- same season meaning as ballots.season (the roster season, schema.md §10)
  submitted_at TEXT NOT NULL
);

CREATE INDEX idx_ballot_submissions_user_weight_season
  ON ballot_submissions(user_id, weight_class, season, submitted_at DESC);

-- One ranked slot on a submission — same shape as ballot_entries, copied
-- verbatim at submit time rather than referencing ballot_entries directly,
-- so a later edit to the live (still-rolling) ballot never rewrites history.
CREATE TABLE ballot_submission_entries (
  id            INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  submission_id INTEGER NOT NULL REFERENCES ballot_submissions(id) ON DELETE CASCADE,
  rank          INTEGER NOT NULL CHECK (rank BETWEEN 1 AND 33),
  wrestler_id   INTEGER NOT NULL REFERENCES wrestlers(id),
  UNIQUE (submission_id, rank),
  UNIQUE (submission_id, wrestler_id)
);

CREATE INDEX idx_ballot_submission_entries_wrestler ON ballot_submission_entries(wrestler_id);
