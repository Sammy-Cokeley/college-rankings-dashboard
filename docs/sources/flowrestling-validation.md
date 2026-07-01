# FloWrestling pipeline validation — full 10-weight live container

Date: 2026-06-30. Corpus: live container `14300895` (2025-26 NCAA DI), fetched as
SSR page HTML with a plain `http.Get`. This closes the correctness gap left by
PR #1, which validated only the 2-weight (125 + 285) fixture. Everything below
was run against all 10 weights × 22 editions = **220 editions / 7080 entries**.

## Result

Green. All 220 editions parse; the full container ingests to 220 snapshots with
zero failures and resolves with zero unresolved entries. Two real defects were
found and fixed, and one schema assumption was corrected.

## Findings

1. **Decode bug (fixed).** `containerFromTransferState` unmarshalled the whole
   Angular transfer-state map into one strongly-typed `map[string]struct{body…}`.
   The live blob has **8 keys, 2 of them bare strings** (config/version), so the
   whole-map unmarshal failed — a path the 2-weight fixture never exercised
   (fixture tests call `ParseContainer` directly, skipping decode). Fix: decode
   value-by-value with `json.RawMessage`, type only the matched
   `ranking-containers` entry. Regression-guarded by heterogeneous values added
   to `decode_test.go`'s synthetic page.

2. **Tie rank → schema change.** 197, 2026-03-27 publishes **two wrestlers at
   rank 21** (Andrew Reall / Brown, Dillon Bechtold / Bucknell — the table then
   skips to 22, then 24; likely an editorial typo). This falsifies the "Flo
   doesn't tie individual weights" assumption. The old
   `UNIQUE(snapshot_id, rank)` rejected the second row and, because an edition
   ingests in one transaction, **discarded the entire 33-row snapshot** over one
   tie — violating "never block ingestion / never lose raw". Fix: relax to
   `UNIQUE(snapshot_id, rank, raw_source_string)` (still catches an exact
   duplicate row), store the published rank verbatim, treat rank as non-unique
   downstream. Rationale recorded in `schema.md` §7. Rejected the alternative of
   deriving rank from row position — that silently rewrites the source and turns
   a loud/rare/detectable anomaly into a silent/systemic one.

3. **Table shapes hold across the 8 previously-untested weights.** Header-driven
   parser: **0 parse errors, 0 blanks, 0 unexpected rank gaps** across all 220
   editions. The only structural anomaly in the whole container is the one tie
   above. No extra columns, footnotes, or "RV" rows appeared.

## Independent cross-checks

- **Movement vs Flo's own `Previous` column:** 6421 comparisons, **6420 agree**.
  The single miss (157, 2026-01-05, Dylan Evans; Flo shows `24 (165)`) is a
  cross-weight annotation our per-weight `LAG` model intentionally doesn't track
  — not an error. Strong confirmation of `MovementForWeight`.
- **Resolution / transfers:** 6 normalized names appear under >1 school. Five are
  genuine transfers (Carter Young, James Conway, Patrick Brophy, Teague Travis,
  Wynton Denkins) — correctly fragmented into 2 identities each, by design. One,
  `Evan Mougalian → Penn / Pennsylvania`, is a **false split** from a school
  abbreviation (same school, two spellings), not a transfer. Every failure mode
  is safe over-splitting; **no two distinct wrestlers were merged**. Counts:
  7080 entries → 520 wrestlers / 521 aliases / 521 distinct raw (name, school)
  pairs (normalization merged exactly one drifted spelling).

## Artifacts

- Fix: value-by-value transfer-state decode (`scraper/decode.go`).
- Schema: `db/migrations/0001_init.sql`, `docs/schema.md` §7.
- Fixture: `testdata/ranking_container_14300895_10weights.json` (all 10 weights)
  as the permanent CI validation corpus, guarded by `TestFullContainer_*`.

## Known follow-ups

Both items below were deferred out of the validation work and have since been
addressed on `feat/flo-pipeline-followups`:

- **School canonicalization (done).** The `Penn`/`Pennsylvania` false split is
  fixed by a curated normalized-school map (`resolve/schools.go`) applied to the
  identity key only — `raw_school` stays verbatim. Merges Mougalian and nothing
  else (520 → 519 wrestlers); the five real transfers still fragment. The map is
  single-source (Flo); a second source will need it to grow, and likely a proper
  `schools`/`school_aliases` dimension keyed on `school_id`.
- **Non-fatal ingest anomaly signal (done).** `ingest.Result` gained an
  `Anomalies` channel (distinct from the fatal `Failures`) that flags duplicate
  ranks (ties) and rank gaps per edition; `cmd/scrape` logs them but exits 0.
  The 10-weight fixture reports exactly the one 197 2026-03-27 tie.
