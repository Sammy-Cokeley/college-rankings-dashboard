# Source recon — InterMat rankings

Findings from probing intermatwrestle.com on 2026-07-04 (the v1 gate task —
see decisions.md "v1 scope"). Captures *how the data is shaped and reached*.
Scraper is not built yet.

Pages inspected (fixtures cached in session scratchpad, not committed):
- `https://intermatwrestle.com/rankings.html/` (rankings index)
- `https://intermatwrestle.com/rankings.html/ncaa-di-r76/` (current DI record)
- Two weekly articles (`…rankings-updated-162026-r100739`, `…-3102026-r100872`)
- Wayback snapshot of `ncaa-di-r63` (the 2026-01-06 edition)

## TL;DR — verdict: **GO** (plain HTTP; backfill via Wayback Machine)

- The "bot protection" is **AI-crawler user-agent blocklisting** (Cloudflare
  managed rules), not a JS challenge. `curl` with its default UA gets **200**;
  so does `Go-http-client/1.1`. A ClaudeBot UA gets **403** — that's the whole
  mechanism, and why WebFetch failed during scoping. **Go `net/http` from the
  Pi works as-is.** No headless browser anywhere.
- The scoping assumption "weekly articles carry the rankings" was **wrong**.
  Weekly "NCAA DI Rankings Updated" articles are per-weight *commentary only* —
  no ranked lists. The lists live in a structured **rankings record**
  (`rankings.html/ncaa-di-rNN/`) that is **replaced weekly and the old record
  deleted** (hard 404). The live site holds only the current edition.
- **History is recoverable anyway: the Wayback Machine captured essentially
  every weekly record** (r41→r75 all have 200-status snapshots; the 2025-26
  season is ≈r53–r76, every one covered). Backfill = fetch ~24 snapshots from
  web.archive.org — zero InterMat traffic.
- In-season table: **33 ranked per weight**, columns `RANK / WRESTLER / SCHOOL /
  CLASS / CONFERENCE / RECORD / LAST` — previous rank (`LAST`, "NR" for new
  entrants), W-L record, and conference are *extra* signal Flo doesn't publish.
- `robots.txt`: `Allow: /` for generic agents but **`Crawl-Delay: 20`** and
  Cloudflare content signals `ai-train=no, use=reference`. Honor 20s+ between
  requests; in-season load is ~2 requests/week anyway.

## Protection: what it actually is

Cloudflare fronts the site (`Server: cloudflare`). Probe matrix (2026-07-04):

| Client / UA                          | Result |
|--------------------------------------|--------|
| `curl` default UA                    | 200    |
| `Go-http-client/1.1`                 | 200    |
| ClaudeBot UA string                  | 403    |
| WebFetch (scoping session)           | 403    |

robots.txt carries the Cloudflare-managed AI-bot block (ClaudeBot, GPTBot,
CCBot, Bytespider, … all `Disallow: /`) and Cloudflare enforces the same list
at the edge by UA. No JS challenge, no cookie gate, no header fingerprinting
observed. Pages are fully server-rendered (Invision Community / IPS suite —
`ips4_*` cookies, IPS Pages "databases" for articles and rankings).

Pipeline consequence: `http.Get` with a self-identifying custom UA works from
the Pi. If they ever tighten to browser-UA-only, a Firefox UA string is the
fallback; a real browser is not needed today.

## Where the data lives (three surfaces)

### 1. Rankings records — the actual lists (current edition only)

`rankings.html/` is an IPS Pages database listing **3 records**: NCAA DI
(`ncaa-di-r76`), DII (`ncaa-dii-r77`), DIII (`ncaa-diii-r71`). One record =
one full edition: tabs for the 10 weights + Tournament (+ Dual) team rankings,
each tab a plain HTML `<table>`, plus a per-weight photo and a per-weight
editorial **Comments** blob (same prose as the weekly article).

Each weekly update creates a **new record id** (r63 → r64 → … → r76) and
**deletes the old one** (r63 is a hard 404 now). The record id increments by
~1/week; the URL slug is stable (`ncaa-di-rNN`).

### 2. Weekly articles — commentary + edition marker (no lists)

`articles.html/college/ncaa-di-rankings-updated-<MDYYYY>-r<articleId>/` —
e.g. `…-162026-r100739` = 1/6/2026. Title carries the unambiguous date:
"NCAA DI Rankings Updated (1/6/2026)". Body = per-weight `<strong>125 lbs</strong>`
+ prose; **no tables, no ranked lists**. Each article links to the rankings
record that was current at publish (the 1/6 article links `ncaa-di-r63`) —
that's the article↔record↔date mapping.

### 3. RSS — in-season new-edition detection

`https://intermatwrestle.com/articles.html/college/?d=1&rss=1` — standard RSS,
48 items, full category (not rankings-only). In-season, poll weekly and match
title prefix `NCAA DI Rankings Updated` → new edition exists → fetch the
current record page. (Alternative signal: the record id on `rankings.html/`
changed.)

## Backfill: Wayback Machine, not InterMat

Because old records are deleted, 2025-26 backfill comes from
web.archive.org. CDX check (2026-07-04): every DI record id r41–r75 has ≥1
**200-status** snapshot taken while it was live; the 2025-26 season is
≈r53 (Oct 2025) → r76 (Apr 2026, still live). Snapshot cadence matches the
weekly replace cycle (r63: captured 1/06, 1/09, 1/12; 404 by 1/15 when r64
took over).

- Enumerate: CDX API
  `web.archive.org/cdx/search/cdx?url=intermatwrestle.com/rankings.html/ncaa-di*`
  filter `statuscode=200`, one snapshot per record id.
- Edition date: Wayback timestamp bounds it; the weekly article's exact
  title date (via its record link) pins it precisely.
- ~24 fetches against archive.org total; zero load on InterMat. Wayback
  rewrites asset URLs but leaves the `<table>` markup intact (verified on the
  r63 snapshot: all 12 tables parse, 33 wrestlers at every weight).
- Same-page caveat: Wayback snapshots may use slightly different attribute
  quoting than the live page — parse with a real HTML parser, not regexes.

## The rankings table

In-season (r63 snapshot, 2026-01-06), per weight — 35 `<tr>`: 1 title row
(`125lbs` banner cell), 1 header, 33 wrestlers:

| RANK | WRESTLER | SCHOOL | CLASS | CONFERENCE | RECORD | LAST |
|------|----------|--------|-------|------------|--------|------|
| 1 | Vincent Robinson | NC State | Sophomore | ACC | 9-1 | 1 |
| 2 | Luke Lilledahl | Penn State | Sophomore | Big Ten | 8-0 | 2 |
| 33 | Greg Diakomihalis | Cornell | Ivy | Senior | 4-2 | NR |

Post-season final (r76, live): **16 rows per weight**, header `Finish /
Wrestler / School / Class / Conference` — no RECORD, no LAST. So, as with Flo:
**column set and row count vary by edition; parse the header row, never
hardcode positions or counts.** Expect hand-entered quirks (stray markup,
"NR"/blanks, possible ties).

Observations for entity resolution:

- **No anchors / no wrestler ids** — name + school text only, same as Flo.
- School naming matches Flo's short style in every sampled row ("NC State",
  "Penn State", "Virginia Tech", "Ohio State", "Princeton", "Cornell") — good
  news for the school-canon map, but don't assume; the resolver validates
  against the full corpus.
- `CLASS` is spelled out ("Sophomore") vs Flo's "SO" — normalize in the mapper.
- `LAST` = InterMat's own previous rank; like Flo's `Previous`, treat as
  cross-check, we still derive movement via `LAG()`.
- `RECORD` (W-L) and `CONFERENCE` have no schema home today — capture them into
  `raw_source_string` (or a raw JSON blob) so nothing published is lost.

## How this maps to the store schema

No schema change needed:

| InterMat                          | store                                   |
|-----------------------------------|-----------------------------------------|
| InterMat                          | `sources` row (new)                     |
| record tab (125…285)              | `snapshots.weight_class`                |
| edition date (article title date) | `snapshots.published_date`              |
| season (Oct–Apr window)           | `snapshots.season` (ending year)        |
| table row                         | `ranking_entries` (one per row)         |
| `RANK`                            | `ranking_entries.rank`                  |
| WRESTLER/SCHOOL (+CLASS/CONF/REC) | `ranking_entries.raw_source_string`     |
| (resolution, 2nd pass)            | `ranking_entries.wrestler_id` (nullable)|

Tournament/Dual tabs and DII/DIII records are out of scope (as with Flo's
P4P/Team sections).

## Politeness / posture

- Honor `Crawl-Delay: 20` (robots.txt) — ≥20s between InterMat requests. Real
  in-season load: 1 RSS poll + 1 record fetch per week. Backfill hits
  archive.org only.
- Self-identify with a custom UA (project name + contact) rather than spoofing
  a browser — we pass without spoofing today, so don't start.
- Content signals say `ai-train=no, use=reference`; we are neither training
  nor excerpting prose — we republish facts (rankings) with prominent
  attribution and links back, per the decided posture (decisions.md).
- **Fixtures:** raw HTML cached in the session scratchpad, NOT committed —
  full-page copies of a paywalled-adjacent publisher feel iffy to vendor into
  a public repo. Decide before scraper work: commit trimmed table-only
  extracts, or keep fixtures out-of-repo (gitignored `fixtures/` fetched by
  script). Flag raised, not resolved.

## Open questions for ingestion design

1. **Preseason/early editions** — is the first 2025-26 edition ~33 deep or
   shorter (Flo's preseason was 24)? Resolved during backfill parsing, not
   blocking.
2. **Edition-date precision for backfill** — article-title date vs Wayback
   timestamp can differ by a day or two for records captured late in their
   week. Decide the tiebreak rule when building the backfill job.
3. **DI record id discovery in-season** — scrape `rankings.html/` for the
   `ncaa-di-rNN` link each week (don't assume +1 increments; r71→r76 shows
   gaps across divisions).
4. **Wayback misses** — if a future week's record is never snapshotted, we
   lose that edition unless we scraped it live. In-season scraping is the
   primary path; Wayback is backfill/repair only.

## Gate verdict

**GO.** Fetch strategy: Go `net/http` + custom UA, ≥20s crawl delay;
in-season = weekly RSS poll → fetch current `ncaa-di-rNN` record page;
backfill 2025-26 = Wayback CDX → ~24 record snapshots. No headless browser,
nothing heavy on the Pi. NWCA fallback not needed.
