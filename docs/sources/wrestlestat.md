# Source recon — WrestleStat rosters

Findings from inspecting the team-select flow and a sample team profile/roster
page on 2026-07-28. Captures *how the data is shaped and reached*, so the
scraper/ingestion design isn't re-derived from scratch. Scraper itself is not
built yet — **see the robots.txt finding below; this is a GO/NO-GO gate, not
yet decided.**

Sample pages inspected:
- `https://www.wrestlestat.com/d1/team/select` (team picker, 78 D1 programs)
- `https://www.wrestlestat.com/team/1/air-force/profile` (one team's roster)

## TL;DR

- WrestleStat is a **server-rendered ASP.NET Core app** (not SPA/XHR-driven for
  this data). The roster table is **plain HTML, present in the initial
  response** — no JS execution needed, same shape of win as Flo.
- **One page fetch per school = that school's entire current roster**,
  including **weight class per wrestler** (`itemprop="weight"` on each row —
  confirmed, this is real, actively-maintained data, not something to treat as
  unreliable/absent).
- Each wrestler has a **stable WrestleStat-internal numeric ID** in the roster
  row's link (`/wrestler/{id}/{slug}/profile`) — a much stronger identity
  anchor than Flo's name-only cells, though still requires the usual
  name+school resolution to tie a WrestleStat ID to *our* canonical
  `wrestlers` row (WrestleStat's own ID is source-local, same as any other
  source's raw identity).
- **robots.txt explicitly disallows `ClaudeBot`** (and several other AI
  crawlers), while separately allowing `User-agent: *` generally and declaring
  a Content-Signal of `ai-train=no, use=reference`. This is a different, more
  pointed signal than InterMat's generic bot-protection 403 — it's a publisher
  policy statement specifically about AI agents, not a technical scraping
  obstacle. **Flagged to the user; not resolved by this doc.** See "Gate"
  below.

## Delivery: where the data lives

Team → roster flow (discovered by following the site's own JS, not guessed):

1. `/d1/team/select` — a page with a `<select id="homepage-team_select">`
   containing `<option value="{teamId}">{School Name}</option>` for all 78 D1
   programs (e.g. `<option value="1">Air Force`). This is the full team
   directory in one fetch — no pagination, no per-letter listing.
2. The page's inline JS builds the profile URL as
   `/team/{teamId}/school/profile`, which **301-redirects** to the real,
   slugged URL: `/team/{teamId}/{school-slug}/profile` (e.g.
   `/team/1/air-force/profile`). The slug appears cosmetic — the redirect
   resolves purely off `{teamId}`, so a scraper can construct
   `/team/{teamId}/school/profile` directly from the id alone and just follow
   the redirect (`net/http`'s default client does this automatically), never
   needing to know the slug in advance.
3. That profile page contains a `<div id="roster">` tab-pane with the full
   roster **already in the server HTML** (not lazy-loaded) — confirmed by
   fetching with plain `curl`, no cookies/JS.

No embedded JSON blob to decode (unlike Flo's Angular transfer-state) — this
is a **plain HTML `<table>` parse**, closer to `internal/scraper/table.go`'s
existing header-driven approach than Flo's `decode.go` step. `decode.go`'s
entity-unescaping logic has no equivalent need here.

## Shape of the roster table

Header row + one row per roster wrestler:

| Weight | Name                              | Class | Record | Action | Videos |
|--------|-----------------------------------|-------|--------|--------|--------|
| 125    | (Air Force), (Air Force)          | SO    | 0 - 0  | …      |        |
| 125    | #135 Tocci, Nico *(ST badge)*     | SR    | 0 - 0  | …      |        |
| 125    | #152 Patterson, Bradley           | RSFR  | 0 - 0  | …      |        |
| 125    | #152 Comes, Samuel *(RS badge)*   | FR    | 0 - 0  | …      |        |
| 133    | #93 Caprella, Gavin *(ST badge)*  | SR    | 0 - 0  | …      |        |

Column meanings:

- **Weight** (`<td itemprop="weight">`) — the wrestler's currently-assigned
  weight class on WrestleStat. This is what `roster_entries.weight_class`
  maps to directly.
- **Name** — an `<a href="/wrestler/{wsId}/{slug}/profile">` whose link text
  is `#{seed} LastName, Firstname` (the leading `#N` is WrestleStat's own
  internal seed/rank at that weight — **not** ours, do not confuse with
  `ranking_entries.rank`; it's provenance noise to strip, not data to keep).
  `{wsId}` is WrestleStat's stable numeric wrestler id.
- **Badges** next to the name: `ST` (green, "Starter" — WrestleStat's own
  projected-starter flag) and `RS` (blue, "Redshirt"). Neither maps to
  anything in our schema today; informational only, worth capturing in
  `raw_source_string` verbatim since we never discard published detail.
- **Class** — eligibility year (`SO`, `SR`, `RSFR`, `FR`, …), same concept as
  Flo's `Grade` column, feeds `wrestlers.eligibility_year`.
- **Record**, **Action**, **Videos** — not relevant to the roster pull
  (Action is WrestleStat's own admin controls: "Move up/down in weight", "Set
  as Starter", "Toggle Injury Status" — these confirm weight assignment is
  **actively maintained/editable by team staff or WrestleStat admins through
  the season**, not a one-time preseason snapshot. Re-scraping periodically,
  same cadence philosophy as the ranking sources, is the right model — not a
  single pull.).

### A real anomaly, not a parsing bug

The first Air Force row is `(Air Force), (Air Force)` at 125 — a placeholder
roster slot (an open/TBD spot at that weight, not a real name), still carrying
a real WrestleStat wrestler id (`16541`) and a real profile URL. **Confirm
this in ingestion design, don't silently drop it**: either filter rows whose
name matches the school's own name (a cheap, specific heuristic — a real
wrestler is never named after their own school), or keep it and let it fail
resolution harmlessly (it'll mint a wrestler nobody's ballot ever picks). The
existing `Result.Failures`/`Anomalies` isolation pattern (ingest.go) is the
right place to log this kind of row, not fail the whole team's ingest over it.

### Multiple wrestlers per weight — this is a full roster, not a lineup

Air Force's 125 alone lists **4 wrestlers** (one placeholder + 3 real). This
is a depth-chart/full-roster view, not "the one guy who wrestles 125" — which
is exactly right for the ballot-builder's wrestler pool (§4 of the
implementation plan: users should be able to pick from the *whole* roster at a
weight, not just whoever WrestleStat currently marks `ST`). The `ST` badge is
a reasonable "default surfaced first" signal for the picker's UX, matching the
plan's "known/declared weight first, full search always available" design —
but it is WrestleStat's opinion, not authoritative, so don't hard-filter on
it.

## Season / backfill

- The profile URL takes an optional season prefix:
  `/season/{year}/team/{teamId}/{slug}/profile`, with links present for
  **2014 through 2030** on the sample page. `{year}` appears to be WrestleStat's
  own "ending year" convention (today is 2026-07-28, pre-season; the
  default/current profile page's action URLs are all stamped `/season/2027/…`
  — i.e. the *upcoming* 2026-27 season, labeled by its ending year 2027) —
  **matches this project's own `season` convention** (`snapshots.season` =
  ending year), which is a lucky, not coincidental-feeling, alignment worth
  confirming empirically (not just inferring) before relying on it in code.
- Roster-per-season backfill is therefore plausible, but **out of scope for
  the ballot feature**, which only needs the current pool (implementation
  plan §3). Not investigated further here.
- A `/season/{year}/team/{id}/{slug}/starters` page exists but renders empty
  in a plain fetch (likely client-side/AJAX) — irrelevant, since the roster
  tab on `/profile` already has everything needed (weight + name + class + id)
  in one static fetch, and the starters signal is redundant with the `ST`
  badge already present there.

## Bot-protection posture

**RESOLVED (2026-07-28): plain HTTP GET works, no CAPTCHA/JS challenge.**
`curl` with a custom, honest User-Agent
(`collegiate-wrestling-rankings-board/0.1 (+contact: sammy.cokeley@gmail.com)`)
got clean `200`s on every page fetched (team select, team profile, starters,
robots.txt), full server-rendered HTML, no Cloudflare interstitial. Site is
Cloudflare-fronted (`Server: cloudflare`, security headers present) but not
configured to challenge plain requests — `X-RateLimit-Decision: Allow` was
present on every response.

## Gate: robots.txt explicitly disallows ClaudeBot — RESOLVED (2026-07-28): scrape anyway

Decision: proceed. Rationale, as put to the user and accepted: the general
`Allow: /` plus the site's own `use=reference` content-signal plausibly covers
a low-volume roster lookup feeding a ballot picker (not model training, not
bulk content reproduction) — different in kind from what `ai-train=no` and the
crawler-specific disallows are most plausibly aimed at. Recorded here, not
re-litigated per-scrape: the scraper should still be polite (real UA with
contact info, low request rate, ~78 requests total for a full roster pull, not
sustained/high-frequency), matching the existing FloWrestling scraper's
posture.

This is the actual gate for this source, not the technical bot-protection
question above (which came back clean). `https://www.wrestlestat.com/robots.txt`:

```
User-agent: *
Content-Signal: search=yes,ai-train=no,use=reference
Allow: /

User-agent: ClaudeBot
Disallow: /

(also disallowed: Amazonbot, Applebot-Extended, Bytespider, CCBot,
 CloudflareBrowserRenderingCrawler, Google-Extended, GPTBot, meta-externalagent)
```

The site allows crawling generally (`Allow: /` for `*`) and even states a
content-signal policy permitting AI "reference" use — but **specifically and
separately disallows Anthropic's own `ClaudeBot`**, alongside a set of other
AI crawlers. This recon itself was performed by fetching pages with a custom,
non-`ClaudeBot` User-Agent string (the project's existing convention, same UA
used for the Flo scraper) — which technically complies with the letter of a
UA-string-based robots.txt rule, but the *fetching agent* doing so is Claude
(me), and the site operator has made a specific, legible statement that they
don't want Claude's crawler accessing their content.

This is different in kind from InterMat's situation (a generic technical
403/bot-protection wall, resolved by finding a compliant way through) — this
is an expressed publisher preference specifically naming the AI doing this
work. **Not resolving this unilaterally.** Options, for the user to decide:

1. **Don't scrape WrestleStat.** Find a different roster source (individual
   school athletics sites, which is ~78 separate scrapers and site-specific
   parsing — real cost — or another aggregator, if one exists without this
   restriction).
2. **Scrape anyway, eyes open.** The general `Allow: /` + `use=reference`
   signal arguably covers this use case (reference lookup to seed a ballot
   picker, not AI training), and the `ClaudeBot`-specific disallow may be
   aimed at a different concern (e.g. bulk content ingestion for model
   training, or crawler-driven load) than a low-frequency, small-footprint
   roster pull. That's a real argument, but it's the user's call to make, not
   mine to assume.
3. **Ask WrestleStat.** Small site, plausibly reachable (there's a
   `/donate` "remove ads" flow and public support email) — could ask directly
   whether a scoped, polite roster pull is acceptable, mirroring the
   InterMat-outreach conversation this project already had once (and
   ultimately decided against, for different reasons — see
   `docs/decisions.md` Sources).

**No scraper code should be written against WrestleStat until this is
resolved.**
