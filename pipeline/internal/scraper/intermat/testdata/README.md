# testdata — InterMat fixtures

Three **trimmed HTML extracts** of real InterMat NCAA DI rankings-record
pages — two from Wayback Machine snapshots (fetched/trimmed 2026-09-12), one
recovered directly from the live site before it was replaced (fetched
2026-09-12, trimmed 2026-09-19). Unlike Flo's testdata (full trimmed JSON
containers), these are table-only excerpts, not full page copies — the repo
is public, and InterMat's full page markup/content isn't something to vendor
in wholesale. All three are consumed via `ParseRecord`.

- **`record_r53_2weights.html`** — the 2025-26 season's very first DI record
  (`ncaa-di-r53`, Wayback capture `20250905143142`, real publish date
  2025-09-05). **Preseason shape**: RECORD is "0-0" for everyone (no matches
  played yet) and there is **no LAST column at all** (nothing to compare
  against in the season's first week) — 6 header columns, not 7.
- **`record_r63_2weights.html`** — a mid-season record (`ncaa-di-r63`,
  Wayback capture `20260106175039`, real publish date 2026-01-06). **Full
  column set**: RANK/WRESTLER/SCHOOL/CLASS/CONFERENCE/RECORD/LAST, 7 columns.
  Matches the sample row in `docs/sources/intermat.md` exactly (rank 1
  Vincent Robinson, NC State, 9-1, previously ranked 1st).
- **`record_r76_2weights.html`** — the real **postseason-final** edition
  (`ncaa-di-r76`), never captured by Wayback (see below) but recovered by
  fetching it directly while it was still live, one week before InterMat
  finally replaced it for the new season. **16 rows, `FINISH` header, no
  RECORD/LAST** — and a wrinkle not anticipated until this fixture: the
  bottom 8 of each weight's 16 finishers carry a non-numeric bracket code
  (`R12`/`R16`) instead of a placement number — see `finishCodeRanks` in
  `table.go`.

All three are trimmed to **2 weight tables** (125 + 285 — lightest and
heaviest, same convention as `scraper/testdata`'s Flo fixture), real markup
preserved verbatim (including the weight-class title banner row every table
carries — `<tr><td colspan="N"><font size="5">125lbs</font></td></tr>` —
which `ParseRecord` reads to identify each table's weight class).

## How they were produced

1. CDX query: `https://web.archive.org/cdx/search/cdx?url=intermatwrestle.com/rankings.html/ncaa-di*&filter=statuscode:200&collapse=urlkey` — confirmed the real response shape (plain space-separated text, not JSON) and located the 2025-26 season's record-id range (r53 through r75; every weekly record from 2025-09-05 through 2026-03-10 has a 200-status snapshot).
2. Fetched two full snapshot pages via `https://web.archive.org/web/<timestamp>/<original-url>`.
3. Trimmed each to just the `<table class="ipsTable...">...</table>` blocks for 125lbs and 285lbs (identified by locating each weight's banner-row text), dropping the other 8 weight tables, Tournament/Dual tables, and all surrounding site chrome (nav, photos, comments, Wayback's own toolbar).

## Known structural variation captured across these fixtures

| edition | date              | header columns                                              | rows/weight |
|---------|-------------------|---------------------------------------------------------------|-------------|
| r53     | 2025-09-05        | RANK / WRESTLER / SCHOOL / CLASS / CONFERENCE / RECORD (**no LAST**) | 33 |
| r63     | 2026-01-06        | RANK / WRESTLER / SCHOOL / CLASS / CONFERENCE / RECORD / LAST | 33 |
| r76     | ~2026-03-24 (est.)| FINISH / WRESTLER / SCHOOL / CLASS / CONFERENCE (no RECORD/LAST) | 16, ranks 9-16 via R12/R16 codes |

This is why `ParseTable`/`ParseRecord` must be header-driven (read column
order/labels from the header row itself), never assume a fixed column set —
same rule as Flo's parser, for the same reason.

## Three more real quirks these fixtures preserve

- **A stray trailing empty `<tr>`** at the very end of r53's 285lbs table (no
  `<td>`s at all — real InterMat markup, not a trimming artifact). Treated as
  structural padding and dropped before row parsing (`directRows` in
  `htmlutil.go`) rather than surfacing as a ragged row — there's no
  `raw_source_string` in an empty row to lose.
- **A genuine hand-entry gap**: r53's 125lbs rank 19 (Brendan McCrone) is
  published with no RECORD cell at all (5 cells where the header has 6) —
  confirmed against the raw Wayback capture, not a trimming mistake, and
  confirmed permanent by checking all 3 real captures across r53's entire
  live period (Sept 5 → Oct 16) — the cell is blank in every one.
  `ParseRows` fails loud on this, exactly per the header-driven-parsing rule
  above; it's a genuine `ingest.EditionFailure` for 125lbs in this edition
  once real backfill runs, not a bug in the parser. 285lbs in the same record
  has no such gap and parses cleanly — see `record_test.go`. **This fixture
  intentionally keeps the row broken** to exercise that fail-loud path; the
  live DB has since been patched with a manually-confirmed value (0-0, from
  the user) and re-ingested — see `docs/sources/intermat.md`'s "Update"
  section. Don't "fix" this fixture to match — it would delete the test
  coverage for real ragged-row handling.
- **Non-numeric FINISH codes** in r76: only ranks 1-8 (the All-Americans) are
  plain digits — the other 8 of each weight's 16 rows read `R12` or `R16`
  (an NCAA bracket-elimination code, not a placement number). Mapped to a
  tie-group rank (`finishCodeRanks` in `table.go`) rather than failed —
  real, meaningful data, not a malformed row.

## A Wayback gap, recovered a different way

The recon doc (`docs/sources/intermat.md`) expected a **postseason-final**
edition (16 rows, `FINISH` header instead of `RANK`, no RECORD/LAST) to be
recoverable too. Checking the real CDX index (2026-09-12) found Wayback never
captured it: the last 200-status DI capture for the 2025-26 season is
r63 → ... → **r75** (2026-03-10), and **r76 has zero Wayback captures at
all** — matches the recon doc's own flagged risk #4 ("if a future week's
record is never snapshotted, we lose that edition"). But r76 turned out to
still be **live** on intermatwrestle.com the same day (2026-09-12) — InterMat
hadn't yet replaced it with the new 2026-27 season — so it was fetched
directly instead of via Wayback and saved immediately, since a live-but-
unarchived page is exactly the kind of thing that can disappear at any time
(it did: by 2026-09-19 the live URL was 404, replaced for the new season —
the saved copy is now the only surviving source). See
`cmd/backfill-intermat`'s `-recover-file`/`-recover-url` flags, added for
exactly this situation. **The published date (2026-03-24) is an estimate**,
not derived from the page itself (it carries no date) — inferred from the
2026 NCAA DI finals wrapping up 2026-03-21/22 (cross-referenced via
InterMat's own tournament-results articles); precise enough for correct
week-over-week ordering, not claimed as exact.

## If you need more

Every 2025-26 DI record (r53 → r75, ~20 of them) is still fetchable from
Wayback the same way — see the CDX query above. Re-fetch if a future change
needs a wider validation corpus; these two cover the structural variation
that matters for the parser today.
