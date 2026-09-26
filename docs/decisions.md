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
   **Resolved (2026-09-24):** tables wrap in a horizontally-scrollable
   container with identity columns (RK/Wrestler, +WT on the all-weights
   page) pinned via `position: sticky`. Row/school toggles and rankings.vue's
   sort headers — all @click on non-interactive tr/td/th — became real
   `<button>`s, free keyboard access with no custom `@keydown` code.
   `aria-sort`/`aria-pressed`/`aria-expanded` added to reflect state. Not
   done: the trajectory chart's click-to-pin lines have no keyboard path
   either (same underlying `toggleWrestler`, but SVG lines need a different
   pattern — roving tabindex — deliberately left as a follow-up, not
   named in the original roadmap item).
4. Cross-weight movement annotation (see Movement display below)
5. InterMat scraper + resolution; backfill 2025-26 articles as the fixed
   validation corpus, mirroring the Flo approach (see Sources below)
6. Multi-source display design + build (open design problem; stays
   neutral/positive — see Analytics above). **Partially pre-empted
   (2026-07-28):** the Fan Poll feature became this site's second source
   before InterMat did, and its display-integration phase had to solve the
   exact same "one hardcoded source" problem to show poll results at all —
   see `web/utils/sources.ts` (`SOURCES`/`resolveSource`, a `?source=` query
   param every rankings route now honors) and `server/utils/queries.ts`
   (`getSeason` scoped per source, not global — sources don't share a season
   number, e.g. Fan Poll's current roster season vs FloWrestling's completed
   backfill). InterMat's display work should extend that registry (add one
   `SOURCES` entry + a seeded `sources` row), not rebuild source selection.
   The open part still remaining: the actual multi-source UI/UX design (tabs
   were the minimum to prove the plumbing works, not a considered design) and
   whatever cross-source presentation questions come up once InterMat's real
   data exists.
   **Resolved (2026-09-19):** InterMat added to `SOURCES`. The predicted
   cross-source presentation question materialized immediately: InterMat
   publishes Conference and season W-L Record, which FloWrestling's schema
   never needed and which an earlier ingest pass had folded into
   `raw_source_string` (a display-breaking workaround, not a real fix — the
   rankings table renders that field directly as the wrestler's name).
   Given real schema columns instead (`raw_conference`/`raw_record`,
   db/migrations/0005), same pattern as the existing `raw_school`/
   `raw_grade`. The weight-page table shows CONF/RECORD columns only when
   the selected source actually publishes them (InterMat); the all-weights
   overview intentionally keeps its existing compact column set rather than
   growing per-source columns there too.
7. Real home page (after multi-source, so it can show both sources)
   **Resolved (2026-09-24)** — see the Web UI section below.
8. Register ranklines.com; rename + og:image/share meta (see Web UI below)
9. In-season ops: Pi cron, new-season Flo container discovery, weekly InterMat
   article discovery, minimal failure/anomaly notification (surface the
   existing `ingest.Result.Anomalies` signal) + Pi deploy — live before
   October. With a second source published live, ops IS the product being
   live; "a late start backfills" holds for data completeness, not for a
   stranger landing mid-season.
   **Resolved in part (2026-09-20):** scheduling itself is live —
   `.github/workflows/scrape.yml` runs both `cmd/scrape` (FloWrestling) and
   `cmd/scrape-intermat` daily via GitHub Actions, not Pi cron. The "Pi
   cron"/"Pi deploy" framing above predates the 2026-07-27 move off the Pi
   (see Stack below); no PaaS has been picked yet either, so Actions — which
   needs nothing provisioned — is the interim answer until a real deploy
   target exists. Daily, not weekly: `IngestEdition` is idempotent per
   `(source, weight, season, published_date)`, so extra runs are free, and
   InterMat's actual publish day already drifts (docs/sources/intermat.md).
   Each scraper's failure surfaces independently (both always run; the job
   fails if either does) via GitHub's built-in email-on-failure — the
   `ingest.Result.Anomalies` signal itself isn't surfaced anywhere new yet,
   it just rides along in the run's log output. Until `DATABASE_URL` points
   at a real database, every run fails on purpose — there is no production
   DB yet. Still open: new-season Flo container discovery (the workflow's
   `FLO_RANKINGS_URL` repo variable is this season's container URL, hardcoded
   same as before, just relocated) and a real failure notification beyond
   GitHub's default email.
   **Flo rolled over (2026-09-24):** confirmed live — Flo's 2026-27
   container is a new id (16146571, was 14300895), exactly the undiscovered
   "new-season container" case named above. Manually re-pointed for now.
   Surfaced a real parser bug at the same time: the new container's table
   header renamed the eligibility-year column from "Grade" to "Year" (same
   field, values also reformatted "JR" → "3rd"); `headerIndex`
   (`pipeline/internal/scraper/table.go`) now aliases "year" to the same
   required column rather than failing every edition. Both seasons now
   coexist in the DB (2026 and 2027) — see the archive note under Data
   retention below for what that means for the older season's reachability.
 
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

**Flagged for a future pass (2026-09-24) — gaps, not deliberate v1 cuts:**
- **No email verification on signup.** `web/server/api/auth/signup.post.ts`
  requires only email + password (Zod schema) and grants a session
  immediately — anyone can sign up with an email they don't own. No
  `email_verified`/`verified_at` column exists on `users`
  (`db/migrations/0003_users.sql`). Found alongside it: there's no
  password-reset flow anywhere in the codebase either — worth deciding
  together, since a reset flow typically wants verified email as a
  prerequisite anyway (don't build one without the other).
- **No ballot-history / profile view.** No page or endpoint lets a user see
  their own ballots across weight classes in one place today —
  `ballot/[weight].vue` and its `GET`/`PATCH` endpoints only ever handle one
  weight class at a time. Real scope fork before building: the `ballots`
  table is deliberately "rolling, never-locked" (`db/migrations/0004_ballots.sql`)
  — one row per `(user_id, weight_class, season)`, no submission timestamps
  or history rows at all (an explicit simplification when the table was
  designed, not an oversight). So this is either (a) a **current-ballots-
  across-weights view** — cheap, just aggregates existing rows, no schema
  change — or (b) **true submission history over time** — real schema work,
  since nothing versioned is stored today. Shape TBD; needs a decision, not
  an assumption, before implementation starts.
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
- **Launch checklist: courtesy heads-up to Flo/InterMat before going public.**
  _(flagged 2026-09-26.)_ Not a consent request — the outreach-first stance
  above still holds, and asking first risks a "no" that then makes continuing
  to use public data look adversarial instead of neutral. But once, right
  before the site actually goes live (not now — nothing to walk back if it's
  said before launch, unlike after), a low-key "here's what we built, we
  attribute you prominently" email costs little and can't be vetoed the way
  an upfront ask can. Timing is the user's call.
- **Possible future sources raised (2026-09-26), not yet scoped:** WIN
  Magazine and The Open Mat — same recon-gate discipline as InterMat needed
  first (bot posture, data shape, update cadence) before committing either.
  Also raised: "The Barn Session," the user's own family-authored ranking —
  a different category entirely, since it's not a scrape target; adding it
  is an authoring-flow decision (closer to how Fan Poll ballots get entered
  than to a scraper), not a recon gate.
- **WrestleStat: roster source for the Fan Poll ballot builder.** _(2026-07-28
  — see `sources/wrestlestat.md` for the full recon.)_ Not a ranking source —
  feeds the wrestler pool users pick from when building a ballot. Technically
  easy (plain server-rendered HTML, weight class already on every roster row,
  no bot-challenge), but `robots.txt` specifically disallows `ClaudeBot`
  alongside several other AI crawlers, while allowing crawlers generally and
  stating a `use=reference` content-signal policy. **Decision: scrape anyway**
  — a low-volume roster lookup feeding a ballot picker reads as "reference
  use," not the model-training use the disallow most plausibly targets. Not
  re-litigated per-scrape; recorded here once.
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
- **DB: SQLite for v0 → Postgres.** _(SQLite decided 2026-06-28; reversed
  2026-07-27.)_ SQLite's rationale held for v0: tiny data + a single weekly
  batch writer meant a single file, trivial Pi backup, zero ops, and WAL gave
  the Nuxt side unlimited concurrent reads that never blocked the weekly
  writer.
  - *The flip trigger, named in advance, fired.* This doc predicted the exact
    condition that would flip the choice: "if pipeline and web ever stop
    sharing a machine and the DB must be reached over a network, SQLite is
    out." The Fan Poll feature (open public signup, many concurrent user
    writes) forced exactly that — the platform is moving off the Pi to a
    hosted PaaS, and PaaS deployments almost always run pipeline and web as
    separate services with no shared local disk. Not a scope-creep decision;
    the trigger this doc named three weeks earlier is what fired.
  - *Port cost was as advertised.* `schema.md`'s Postgres-deltas section
    (written the same day as the SQLite decision, precisely so this wouldn't
    be a scramble) turned out accurate: `INTEGER PRIMARY KEY` →
    `GENERATED ALWAYS AS IDENTITY`, drop the SQLite pragmas, everything else
    identical. The real cost wasn't the schema — it was every hand-written
    query's placeholder syntax (`?` → `$N`), `LastInsertId()` (no Postgres
    equivalent; became `RETURNING id`), and the web app's DB client going from
    synchronous (`better-sqlite3`) to async (see `web/` — every `server/api/
    rankings/*` route and `server/utils/queries.ts` call site changed).
  - *Driver note, for anyone repeating this:* the obvious Node driver choice,
    `pg`, hit a real, reproducible incompatibility with this project's
    Vite/Vitest toolchain (`pg-pool`'s internal `class BoundPool extends
    Pool` breaks under Vite's oxc transform — "Class extends value [object
    Module] is not a constructor"). Confirmed via a minimal repro before
    switching, not a superstition: swapped to `postgres` (postgres.js), which
    is ESM-native and has no such issue.
  - Two SQLite files (rankings + a hypothetical separate community DB) were
    briefly considered for the Fan Poll's ballots feature specifically, before
    the whole-platform move was decided — superseded once everything is one
    Postgres database: real foreign keys everywhere, no cross-file
    application-level joins needed.
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

## Data retention

- **Nothing is deleted; only the UI is season-scoped.** _(noted 2026-09-24,
  once Flo's 2026-27 rollover made it concrete rather than hypothetical.)_
  `IngestEdition` only ever inserts — every past season's snapshots stay in
  Postgres forever, keyed by `season`. But every rankings route
  (`server/api/rankings/*`) calls `getSeason()`, which hardcodes
  `MAX(season)` per source with no override — so the instant a source rolls
  to a new season, the previous one becomes completely unreachable through
  the UI, even though it's fully intact in the DB.
  - **Not building a season archive yet.** Real scope (a season selector,
    `getSeason` taking an optional override, a URL shape decision for old
    seasons) and, as of the 2026-09-24 rollover, nothing is actually stale:
    Flo just started 2026-27 and InterMat is one edition into the same
    season. The trigger for building this is a *second* rollover, once
    "last season" is real archive material rather than last week's data.
## Web UI
 
- **The "All Weights" dashboard is NOT the intended home page.** _(noted
  2026-07-02.)_ The flat all-weights table at `/` is a useful power-user tool
  but a weak front door; it exists because v0 needed a landing surface, not
  because it won a design. A proper home page (shape TBD — e.g. latest-week
  summary, biggest movers, weight-class entry points) is **v1 work** _(promoted
  2026-07-04)_, sequenced after the multi-source display so it can show both
  sources. Keep the all-weights table reachable when that lands; don't grow
  features into it in the meantime on the assumption it stays the home page.
  **Resolved (2026-09-24):** actually shipped earlier than this note was
  updated — `web/pages/index.vue` landed with the Fan Poll feature (commit
  `1fa2966`, "...a real landing page"): hero with a ballot-building CTA per
  weight class, "Biggest movers this week," and an explainer. Its copy and
  movers predated InterMat going live, so this pass brought it current: the
  movers section now has the same `SOURCES`/`?source=` tabs as
  `rankings.vue`/`[weight].vue` (previously FloWrestling-only), and the
  hero/explainer copy names all three sources. The all-weights table stays
  reachable via "See full rankings" and the topbar, per the original note.
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