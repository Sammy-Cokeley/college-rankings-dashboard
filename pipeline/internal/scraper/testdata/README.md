# testdata — FloWrestling fixture

`ranking_container_14300895.json` — a **trimmed, decoded** capture of the live
FloWrestling season container `14300895` (2025-26 NCAA DI), taken 2026-06-28.

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
