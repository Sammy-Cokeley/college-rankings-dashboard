# Collegiate Wrestling Rankings Board
 
Fan-facing web app presenting NCAA DI wrestling rankings with week-over-week
movement and richer context than the raw ranked lists the sources publish.
 
> Behavioral conventions (terseness, plan-before-code, pushback, no unrequested
> refactoring) are inherited from the global CLAUDE.md and intentionally NOT
> repeated here. This file holds project-specific truth only.
 
## Architecture
 
Monorepo, two decoupled apps sharing one database (a monorepo is a repo layout,
not a runtime architecture — the runtime here is a simple monolith, which is
correct at this scale: small, read-heavy, weekly-updated data).
 
- `pipeline/` — Go. Weekly batch scrapers + entity resolution. Writes to DB.
  Runs via cron. Plays to Go's strengths (concurrency, parsing, single binary).
- `web/` — Nuxt 3 (Vue 3, SSR). Rankings pages server-rendered for SEO and
  shareability; client-side filter/search/sort for the dashboard. Reads the DB
  via Nitro server routes — no separate API layer in v0.
- `db/` — language-neutral SQL migrations. Single source of schema truth.
  Neither app owns the schema.
Full data model: `docs/schema.md`. Product + source + stack decisions and
their rationale: `docs/decisions.md`.
 
## The hard problem
 
Entity resolution, NOT scraping. Sources name wrestlers and schools
inconsistently and wrestlers transfer schools / change weight classes mid-season.
Rules:
- Every ranking entry stores its exact `raw_source_string`. Never discard it.
- Ingest raw first; resolve to a canonical wrestler in a SECOND pass. Never
  block ingestion on resolution (`wrestler_id` is nullable until matched).
- A canonical `wrestlers` entity + a `wrestler_aliases` table reconciles each
  source's raw strings to one identity.
Budget effort here accordingly — it dwarfs the scraping work.
## Stack
 
- Pipeline: Go
- Web: Nuxt 3 (Vue 3, SSR)
- DB: **Postgres** (migrated from SQLite 2026-07-27 — the named flip trigger,
  pipeline and web no longer sharing a machine, fired once the Fan Poll
  feature forced a move off the Pi. See `docs/decisions.md` Stack).
- Deploy: hosted PaaS (moved off the Raspberry Pi 2026-07-27; the Pi remains a
  dev/test device). See `docs/decisions.md`.
## Data sources (v0)
 
- FloWrestling — guest-accessible current *and* full historical weekly rankings.
  One season "ranking container" embeds every weight's dated weekly editions, so
  the same source covers live rankings AND backfill. Recon: `docs/sources/flowrestling.md`.
- Last season's Flo container (2025-26) = launch backfill + the fixed
  scraper/resolver validation corpus. (Separate match-"results" data dropped from
  v0; may return later as ground-truth. See `docs/decisions.md`.)
- InterMat — DEFERRED. Archive is on Rokfin and partly gated; requires a
  relationship, not just scraping. See `docs/decisions.md`.
## v0 scope
 
Out of season → no live data yet. Build and validate scrapers against last
season, backfill it as launch content so the movement feature isn't empty at
launch. Ship: pipeline → snapshot store → SSR display with week-over-week
movement. Hold adversarial cross-source analytics (disagreement, snubs/reaches)
for later — neutral/positive presentation of sources first.