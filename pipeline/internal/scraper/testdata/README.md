# testdata — FloWrestling fixtures

Two decoded captures of the live FloWrestling season container `14300895`
(2025-26 NCAA DI). Both are consumed via `ParseContainer`.

- **`ranking_container_14300895.json`** — trimmed to **2 weights** (125 + 285),
  all 22 editions. The small, *clean* corpus that backs the precise parser /
  ingest / resolve unit tests (contiguous ranks, 1:1 identities). Taken
  2026-06-28. Details below.
- **`ranking_container_14300895_10weights.json`** — **all 10 weights**, all 22
  editions each (220 editions). The full-container validation corpus, added
  2026-06-30 after validating the pipeline against the live page. It carries the
  real-world messiness the 2-weight cut doesn't: a genuine **tie rank** (197,
  2026-03-27 — two wrestlers at 21) and a school-abbreviation identity split
  (`Penn`/`Pennsylvania`). Guarded by `TestFullContainer_*` in the scraper
  package. Prose fields dropped, and the content tables' inline HTML attributes
  (styles) are stripped — the parser reads cell text only, so this is
  parser-invariant and just keeps the file diffable (~1.5 MB, not ~4.3 MB). For
  full-fidelity raw markup handling, see the 2-weight fixture below, which keeps
  Flo's HTML verbatim.

## `ranking_container_14300895.json` (2-weight)

A **trimmed, decoded** capture taken 2026-06-28.

## How it was produced

1. Plain `GET` of a rankings page (`…/14300895-2025-26-ncaa-di-wrestling-rankings/…`)
   returns SSR HTML containing `<script id="flo-app-state">`.
2. That blob is entity-escaped JSON. Reverse the 5 substitutions
   (`&q;`→`"`, `&a;`→`&`, `&l;`→`<`, `&g;`→`>`, `&s;`→`'`) and `JSON.parse`.
3. The ranking container is the value whose transfer-state key contains
   `ranking-containers`, under `.body.data`.

See `docs/sources/flowrestling.md` for the full recon.

## What was trimmed (and why it's still enough)

- **Sections reduced to 125 (key `"2"`) and 285 (key `"11"`)** — lightest and
  heaviest weights. All **22 weekly editions** kept for each, so the full
  temporal range (preseason → postseason) is represented.
- **Editorial prose dropped** per edition (`description`, `seo_description`,
  `asset`, …). Kept: `id, index, name, slug, slug_uri, status, publish_date,
  headline, content`. The `content` HTML `<table>`s are intact — that's the
  data the parser consumes.
- Result: ~700 KB instead of the full 5.7 MB page.

**Known structural variation captured in this fixture** (this is why the parser
must not assume one table shape):

| edition       | "Previous" column | ranked rows |
|---------------|-------------------|-------------|
| 2025-06-19    | **absent**        | 24          |
| 2025-07-02    | present           | 24          |
| 2025-09-29 →  | present           | 33          |

## If you need more

The full page is re-fetchable any time with a plain `http.Get` (no auth, no
browser) — see the recon doc. Re-fetch if you need all 10 weights for a final
cross-weight audit; this fixture covers the temporal/structural variation that
matters for building the parser.
