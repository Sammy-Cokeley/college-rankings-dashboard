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
## v1 scope
 
_(decided 2026-07-04, scoping session. Framing question: what does a stranger
need on day one of the live season for this site to be worth bookmarking over
Flo's own list? Theme: **"first live season, multi-source."** Multi-source
roughly doubles v1 vs the launch set alone — accepted explicitly.)_
 
**In v1** (sequenced):
1. InterMat bot-protection recon — the gate; decides InterMat vs NWCA fallback
2. Minimal .vue component test harness (enabler for the template-heavy work;
   `[weight].vue` fold/selection logic is untested)
3. Mobile pass, with keyboard a11y (sortable headers, row toggles) folded in
4. Cross-weight movement annotation (see Movement display below)
5. InterMat scraper + resolution; backfill 2025-26 articles as the fixed
   validation corpus, mirroring the Flo approach (see Sources below)
6. Multi-source display design + build (open design problem; stays
   neutral/positive — see Analytics above)
7. Real home page (after multi-source, so it can show both sources)
8. Register ranklines.com; rename + og:image/share meta (see Web UI below)
9. In-season ops: Pi cron, new-season Flo container discovery, weekly InterMat
   article discovery, minimal failure/anomaly notification (surface the
   existing `ingest.Result.Anomalies` signal) + Pi deploy — live before
   October. With a second source published live, ops IS the product being
   live; "a late start backfills" holds for data completeness, not for a
   stranger landing mid-season.
 
**NOT in v1** (explicit cuts, not oversights):
- Conference filter on the bump chart — coupled to the curated
  school→conference dimension; build it alongside the `schools`/
  `school_aliases` work the second source forces, as v2 follow-on.
- Wrestler pages — today a thin wrapper over bump-chart `?sel=` selection;
  their moment is after cross-weight + multi-source data matures.
- Adversarial cross-source analytics — stays deferred (above); NOT unlocked
  merely because two sources exist.
- NWCA as a displayed source — fallback only, if the InterMat gate fails.
- InterMat outreach — superseded (see Sources below).
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
- **InterMat: v1 second source — quiet-scrape, published at launch.** _(REVERSED
  2026-07-04; was "deferred". Owner context below still true: MatScouts /
  Willie Saylor.)_ v1-scoping recon changed the technical picture: the full
  2025-26 season of weekly "NCAA DI Rankings Updated" articles is publicly
  listed on intermatwrestle.com — so backfill + a fixed validation corpus are
  public, mirroring the Flo approach; the Rokfin subscription-gating applies to
  *older* history only. Frictions accepted with eyes open: the site 403s
  non-browser clients (active bot protection), and rankings are per-article
  HTML (no container model — per-article scraping, format-drift risk).
  - **Gate: RESOLVED GO.** _(2026-07-04, recon — see `sources/intermat.md`,
    which supersedes the technical picture above.)_ The 403 is AI-crawler UA
    blocklisting only — plain Go `net/http` passes, no headless browser. The
    scoping assumptions were wrong in both directions: weekly articles are
    commentary-only (no ranked lists), and the lists live in a structured
    rankings record that is replaced weekly and *deleted* — but the Wayback
    Machine snapshotted every 2025-26 weekly record, so backfill comes from
    archive.org with zero InterMat traffic, and in-season is ~2 polite
    requests/week (RSS poll + record fetch, honoring `Crawl-Delay: 20`).
    NWCA fallback not needed.
  - **Posture mitigation:** attribute prominently and link every ranking back
    to InterMat — consistent with the neutral-presentation stance above.
- **Outreach-first strategy: superseded.** _(2026-07-04.)_ The prior plan —
  build a finished demo on Flo/public data, then approach Saylor framing the
  site as driving traffic to InterMat, publishing InterMat data only after a
  yes — is no longer a precondition. Decision: scrape the public articles and
  publish at launch, accepting that detected scraper traffic likely poisons any
  future relationship, and that Saylor's competitor partnership means outreach
  was always a disclosure anyway. (Original disclosure analysis kept for the
  record: InterMat's owner is partnered with an established competitor building
  exactly these kinds of boards, so any approach is effectively telling them.)
- Other sources: NWCA Coaches Poll (ncaa.com) is the designated **fallback
  second source** if the InterMat gate fails; The Open Mat remains unplanned.
  Two sources is thin for any "consensus" framing but fine for display.
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
 
- **Movement is per-weight.** _(decided 2026-07-02, with the web v0;
  cross-weight context was deferred then — promoted to v1 below.)_ The
  displayed movement is LAG over
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
- **Cross-weight annotation: promoted to v1.** _(2026-07-04; was a deferred
  enhancement.)_ Not a data gap: the schema already represents weight changes
  fully (weight lives on the snapshot, §3), and resolution ties the weights to
  one canonical `wrestler_id` — the "NEW — previously #24 at 165" annotation
  is one "last ranked at any weight, this source+season" query. Promoted
  because it is the single place Flo's presentation beats ours (their
  Previous column shows `24 (165)`; we show a bare NEW), which fails the
  "worth bookmarking over Flo" test. The known edge cases become acceptance
  criteria, not blockers: unresolved entries have no identity to follow;
  per-source only; "current weight" is ambiguous in the week both weights'
  lists include the wrestler (editions publish on different dates).
## Web UI
 
- **The "All Weights" dashboard is NOT the intended home page.** _(noted
  2026-07-02.)_ The flat all-weights table at `/` is a useful power-user tool
  but a weak front door; it exists because v0 needed a landing surface, not
  because it won a design. A proper home page (shape TBD — e.g. latest-week
  summary, biggest movers, weight-class entry points) is **v1 work** _(promoted
  2026-07-04)_, sequenced after the multi-source display so it can show both
  sources. Keep the all-weights table reachable when that lands; don't grow
  features into it in the meantime on the assumption it stays the home page.
- **Site name: Ranklines (ranklines.com).** _(decided 2026-07-04.)_ Chosen in
  the v1 scoping session from a DNS-checked shortlist (no A record on
  2026-07-04; verify + register at a registrar before the rename/og work —
  DNS absence is a signal, not a guarantee). Named for the product itself: the
  season bump chart's rank lines, which are also the natural logo motif.
  Runners-up: MatMovement, MatTrends, WeighInWeekly. Mockup brands
  ("MatBoard" etc.) were placeholders and are dead. og:image + share meta
  follow the registration.
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