-- InterMat publishes two columns FloWrestling never needed: conference and
-- season W-L record. Nullable and source-optional, same pattern as the
-- existing raw_school/raw_grade — never populated for a source that doesn't
-- publish them (docs/decisions.md).
ALTER TABLE ranking_entries
  ADD COLUMN raw_conference TEXT,
  ADD COLUMN raw_record TEXT;
