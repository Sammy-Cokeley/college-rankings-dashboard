# Source recon — FloWrestling rankings

Findings from inspecting a guest rankings page on 2026-06-28. Captures *how the
data is shaped and reached*, so the scraper/ingestion design isn't re-derived
from scratch. Scraper itself is not built yet.

Sample page inspected:
`https://www.flowrestling.org/rankings/14300895-2025-26-ncaa-di-wrestling-rankings/56108-125-luke-lilledahl`

## TL;DR

- Flo is an **Angular SSR** app. Ranking data is **not** a client-side XHR — it
  is server-rendered and embedded in the page HTML as an Angular transfer-state
  blob (`<script id="flo-app-state">`).
- The blob is **entity-escaped JSON**. Decoded, it contains the backing API
  response: a *ranking container* holding the **entire season** — every weight,
  every weekly edition, each dated.
- Per-wrestler rows are **plain-text HTML `<table>`s** inside the JSON. **No
  wrestler IDs, no profile links** — name + school are text only. Entity
  resolution remains the hard problem.
- One page fetch = the whole season for all weights. Historical movement is
  **backfillable directly from Flo**; we do not have to accumulate week-by-week.

## Delivery: where the data lives

The page boots empty of data on the client; Angular fetched it server-side and
cached it in transfer state. So **DevTools → Network → Fetch/XHR shows nothing
ranking-related** — that is expected, not a dead end.

The data is in the initial HTML document:

```
<script id="flo-app-state" type="application/json"> … </script>
```

The backing API URL appears as the transfer-state key:

```
https://api.flowrestling.org/api/experiences/web/legacy-core/ranking-containers/14300895?site_id=2&version=1.33.2
```

**Calling that API directly returns 404** ("No route found", a Symfony routing
rejection — not auth). Their gateway routes on internal headers the SSR backend
has and we don't. Do not chase the direct API; read the blob from the page.

### Decoding the blob

The blob is JSON with these character substitutions (Angular platform-server
escaping). Reverse them, then `JSON.parse`:

| Encoded | Decoded |
|---------|---------|
| `&q;`   | `"`     |
| `&a;`   | `&`     |
| `&l;`   | `<`     |
| `&g;`   | `>`     |
| `&s;`   | `'`     |

Apply `&a;` → `&` **last** (other tokens decode first). HTML entities inside
editorial text (e.g. `&rsquo;`) survive as literal entities after decoding —
fine, they only appear in prose fields, not in the ranking cells.

The decoded object is keyed by `G.<api-url>`; the ranking container is the value
whose key contains `ranking-containers`, under `.body.data`.

## Shape of the container

`body.data` (the container) — relevant fields:

- `id` — container id (`14300895`), also the leading number in the page URL.
- `title` — e.g. "2025-26 NCAA DI Wrestling Rankings".
- `latest_publish_date`, `latest_ranking_id`, `latest_ranking_slug_uri` — the
  most recent edition pointers.
- `ranking_sections` — **object keyed `"1"`…`"12"`** (not an array). Each value
  is an array of weekly **editions**.

### Sections (the 12 keys)

| key | section          | notes                         |
|-----|------------------|-------------------------------|
| 1   | Pound-For-Pound  | cross-weight; ignore for v0   |
| 2   | 125              | weight class                  |
| 3   | 133              | weight class                  |
| 4   | 141              | weight class                  |
| 5   | 149              | weight class                  |
| 6   | 157              | weight class                  |
| 7   | 165              | weight class                  |
| 8   | 174              | weight class                  |
| 9   | 184              | weight class                  |
| 10  | 197              | weight class                  |
| 11  | 285              | weight class                  |
| 12  | Team Tournament  | team, not individual; ignore  |

The 10 weight sections are what feeds `snapshots`/`ranking_entries`. P4P and
Team Tournament are out of scope for v0 (different entity than a ranked
wrestler-at-weight).

### Editions (each section's array)

For 2025-26: **22 editions**, each an object with:

- `id` — edition id (e.g. `56108`), appears in the per-edition slug URL.
- `name` — the section label ("125", "Pound-For-Pound", …).
- `publish_date` — **ISO date, the canonical time spine** (maps straight to
  `snapshots.published_date`). Series ran `2025-06-19` → `2026-03-27`, weekly
  in-season, sparser preseason.
- `headline`, `description` — editorial prose (the "why" behind the week's
  changes). Not needed for the rankings table; potentially nice context later.
- `content` — **HTML `<table>` of the actual ranked wrestlers** (see below).

> One container fetch yields all 12 sections × 22 editions. For the 10 weights
> that's ~220 dated snapshots and ~7k entries per season — trivial for SQLite.

## The wrestler table (`content`)

`content` is an HTML string: a `<table>` with a header row plus one row per
ranked wrestler. Columns (125, edition 2026-03-27, 33 ranked + header = 34 rows):

| Rank | Grade | Name | School | Previous |
|------|-------|------|--------|----------|
| 1 | SO | Luke Lilledahl | Penn State | 1 |
| 2 | SO | Marc-Anthony McGowan | Princeton | 13 |
| 3 | JR | Nico Provo | Stanford | 6 |
| 4 | SO | Vincent Robinson | NC State | 7 |
| 5 | SR | Eddie Ventresca | Virginia Tech | 2 |

Observations:

- **No anchors / no IDs.** Cells are plain text. `Name` and `School` are the only
  identity we get — this is exactly the reconciliation problem (`wrestlers` +
  `wrestler_aliases`). Plan the matcher around name+school strings.
- **`Grade`** = eligibility (FR/SO/JR/SR). Maps to `wrestlers.eligibility_year`
  (informational), not stored on the entry.
- **`Previous`** = Flo's *own* previous-edition rank. We still derive movement
  ourselves via `LAG()` (schema.md §5); treat Flo's `Previous` as a free
  **cross-check / fallback**, not the source of truth. (Watch for "NR"/blank for
  new entrants.)
- **Table shape varies across editions — do not assume one layout.** Confirmed
  in the 125 section: the first preseason edition (2025-06-19) has **no
  "Previous" column** (nothing prior to compare) and ranks 24; 2025-07-02 adds
  Previous, still 24; from 2025-09-29 on it's Previous + 33 ranked. So both the
  **column set** and the **row count** change over a season. Parse by reading the
  header row to locate columns; never hardcode positions or a row count. (These
  are hand-entered editorial tables — also expect stray markup, "NR"/blanks in
  Previous, and tie ranks: confirmed at 197 2026-03-27, where two wrestlers share
  rank 21 — likely a typo, but stored verbatim. The schema permits it; see
  schema.md §7.)

## How this maps to the store schema

No schema change needed. The natural mapping:

| Flo                              | store                                  |
|----------------------------------|----------------------------------------|
| container (FloWrestling)         | `sources` row (already seeded)         |
| section name → weight (125…285)  | `snapshots.weight_class`               |
| edition `publish_date`           | `snapshots.published_date` (time spine)|
| container season (2025-26)       | `snapshots.season` (ending year, 2026) |
| `content` table row              | `ranking_entries` (one per row)        |
| row `Rank`                       | `ranking_entries.rank`                 |
| row Name/School text             | `ranking_entries.raw_source_string`    |
| (resolution, 2nd pass)           | `ranking_entries.wrestler_id` (nullable)|

## Open questions for ingestion design (not yet decided)

1. **Scraper transport — RESOLVED (2026-06-29): plain HTTP GET works.** `curl`
   of the page returns **200** (with default *and* browser User-Agent) and the
   raw server HTML already contains the full `flo-app-state` blob (~5.7 MB,
   `ranking-containers` + wrestler names present). No headless browser, no JS
   execution, no special headers needed — so nothing heavy on the Pi. Scraper =
   `http.Get` → extract `<script id="flo-app-state">` → reverse the 5 entity
   subs → `json.Unmarshal` → walk `ranking_sections` → parse each edition's
   `content` table. (Be a polite client anyway: set a real UA, low request rate.)
2. **`raw_source_string` contents.** Flo splits Name/School/Grade into cells.
   Decide what we capture verbatim (likely `Name` + `School`, since the entry
   table has no school column) and what feeds the matcher. Never lose the
   published text.
3. **Entry id stability.** Flo edition ids (`56108`) are stable handles for a
   *snapshot*, not a wrestler. Could store for idempotent re-scrapes; not in the
   schema today.
4. **Container discovery.** This season's container id is `14300895`. Need to
   confirm how to discover the current/next season's container id (URL pattern,
   a listing endpoint, or hardcode per season).
```
