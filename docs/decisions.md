# Decisions & Rationale
 
A record of the calls made during planning so they aren't relitigated. Update
when a decision changes; note the date and the reason.
 
## Product shape
 
- **Display-first.** v0/v1 is a presentation tool: show the sources' rankings,
  attributed, with week-over-week movement and context. Not a new ranking.
- **Analytics layered later, neutral before adversarial.** Movement and trends
  (flattering to the sources) come first. Cross-source disagreement, "snubs and
  reaches," source-vs-source — divisive and partner-sensitive — are held until
  the data foundation exists and any source relationships can absorb them.
- **Goal is the fan experience, not personal recognition.** Build something
  fans enjoy; spotlight is not the driver. (Recorded because it shapes how much
  weight to give the relationship/competitive politics below: low, by choice.)
## Sources
 
- **v0: FloWrestling — current *and* full historical weekly rankings.** Available
  now, no permission, no waiting. _(Refined 2026-06-28 after source recon — see
  `sources/flowrestling.md`.)_ Flo's season "ranking container" embeds every
  weight's full weekly edition history, each dated, in one response — so a single
  source covers both live rankings and backfill.
  - **Backfill + validation are the same Flo corpus.** Last season's complete
    container (2025-26: 22 dated weekly editions per weight) is ingested as launch
    content so week-over-week movement isn't empty at launch, *and* serves as the
    fixed scraper/entity-resolution validation set (it never changes, so the
    pipeline can be re-run against a known-good corpus). One dataset, both jobs.
  - **"Last season's public *results*" is no longer a v0 source.** The original
    plan named a separate match-results dataset for backfill/validation; recon
    made it unnecessary — Flo itself carries the historical *rankings*. Match
    results may return later as independent ground-truth to sanity-check rankings,
    but they are out of v0 scope.
- **InterMat: deferred.** Owned by MatScouts (Willie Saylor). Current college
  rankings post on the InterMat site, but the historical archive lives on Rokfin
  and is partly subscription-gated — scraping gated content is worse posture
  than public pages, so InterMat's history realistically needs a relationship,
  not a scraper.
- **Outreach is a disclosure.** InterMat's owner is partnered with an
  established competitor in this space (a builder of exactly these kinds of
  boards). Telling InterMat "I'm building this" is effectively telling that
  competitor. If/when approaching: build a finished-looking demo first (on Flo /
  public data, not on scraped InterMat data), frame it as driving traffic TO
  their rankings, and be ready to move fast afterward. Decision for now: build
  quietly on Flo + public data; revisit InterMat from a position of a working
  product.
- Other sources (NWCA Coaches Poll on ncaa.com, The Open Mat) are public and
  could be added later if a multi-source view is wanted; two sources is thin for
  any "consensus" framing but fine for display.
## Stack
 
- **Monorepo, monolith runtime.** Monorepo = repo layout (correct for a solo dev
  with 2–3 packages); monolith = runtime (correct at this scale — a few thousand
  rows/season, read-heavy, weekly updates). Scaling is a non-issue. No monorepo
  tooling (Nx/Turbo/Bazel) — plain folders + a Makefile.
- **Pipeline: Go.** Concurrency, robust parsing, single-binary cron job.
- **Web: Nuxt 3 (Vue SSR).** Fits the filter/search/sort-heavy dashboard and
  gives SSR for shareable, SEO-friendly pages. Nitro server routes read the DB
  directly — no separate API in v0.
  - *Alternative considered:* Go `html/template` + htmx (single language,
    simpler deploy). Rejected for v0 because the dashboard's snappy client-side
    table filtering fits a reactive front end better. Revisit if the Node
    runtime on the Pi becomes annoying.
- **DB: SQLite for v0.** _(decided 2026-06-28.)_ Tiny data + a single weekly
  batch writer = SQLite is a single file, trivial to back up on the Pi, zero
  ops. WAL mode gives the Nuxt side unlimited concurrent reads that never block
  the weekly writer. Postgres is equally valid and already familiar, but its
  operational cost (a daemon to run/monitor/`pg_dump` on the Pi) buys nothing at
  this scale. Rationale recorded; the store layer, DSN, and schema are already
  SQLite.
  - *The one thing that would flip it:* if pipeline and web ever stop sharing a
    machine and the DB must be reached over a network, SQLite is out → Postgres.
    Nothing in v0 implies that. Port between them is an afternoon (see
    `schema.md` Postgres deltas), so SQLite-now does not lock us in.
- **Decoupling:** pipeline and web share only the DB; `db/` owns the schema as
  language-neutral SQL migrations.
## Movement display
 
- **Movement is per-weight; cross-weight context is deferred.** _(decided
  2026-07-02, with the web v0.)_ The displayed movement is LAG over
  `published_date` partitioned by source/weight/season/wrestler (`schema.md`
  §5). A mid-season weight change therefore shows as either **NEW** (never
  ranked at the new weight) or movement from the wrestler's *last edition at
  that same weight*, however stale. Canonical example: Dylan Evans left 157
  after 2025-10-29 (rank 21), wrestled 165 through December, and returned to
  157 on 2026-01-05 at 16 — we show ▲5 vs his October rank; Flo's own
  "Previous" column shows `24 (165)`, his last rank at the *other* weight.
  This is the single divergence from Flo's column in 6421 comparisons
  (`sources/flowrestling-validation.md`), it's deliberate, and it's pinned by
  an integration test.
- **Deferred enhancement, not a data gap:** the schema already represents
  weight changes fully (weight lives on the snapshot, §3), and resolution ties
  the weights to one canonical `wrestler_id` — so a future "NEW — previously
  #24 at 165" annotation is one "last ranked at any weight, this
  source+season" query. Held out of v0 because it's context enrichment on top
  of week-over-week movement, it grows the API row shape, and it has real
  edge cases (unresolved entries have no identity to follow; per-source only;
  "current weight" is ambiguous in the week both weights' lists include the
  wrestler, since editions publish on different dates).
## Web UI
 
- **The "All Weights" dashboard is NOT the intended home page.** _(noted
  2026-07-02.)_ The flat all-weights table at `/` is a useful power-user tool
  but a weak front door; it exists because v0 needed a landing surface, not
  because it won a design. A proper home page (shape TBD — e.g. latest-week
  summary, biggest movers, weight-class entry points) is future work. Keep the
  all-weights table reachable when that lands; don't grow features into it in
  the meantime on the assumption it stays the home page.
- **Site naming is open.** Mockup brands ("MatBoard" etc.) were placeholders;
  the visual identity work styles the existing descriptive title until a real
  name is chosen.
- **Season trajectory = full-weight bump chart; conference filter deferred.**
  _(2026-07-03.)_ The weight page charts every resolved wrestler's
  rank-over-week line (SSR SVG, no chart library); single wrestlers/groups are
  a selection state (click rows/lines; `?sel=` is shareable), with gaps drawn
  honestly (absence ≠ interpolation). Selecting "all Big Ten wrestlers"
  requires a conference dimension that does not exist yet — the `schools`
  table is empty and only per-entry `raw_school` strings are real. Deferred
  until a curated school→conference mapping is built (likely alongside a real
  `schools`/`school_aliases` dimension, see the resolver's school-canon map).
  School-level selection ships now (raw string grouping is free).
## The hard problem
 
Entity resolution across sources — not scraping. Canonical `wrestlers` +
`wrestler_aliases`; ingest raw, resolve second, never lose `raw_source_string`.
See `schema.md`.