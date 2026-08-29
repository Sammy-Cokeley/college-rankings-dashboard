# web/ — Nuxt 3 front end

Fan-facing SSR app for the rankings board. Reads the pipeline-built Postgres DB
directly via Nitro server routes (no separate API layer in v0). Still only
issues SELECTs today (no write routes exist yet) — see `server/utils/db.ts`
for how that invariant is meant to be enforced once a write path (e.g.
community ballots) exists.

## Setup

Requires Node 24 LTS and a local Postgres (`docker compose up -d` from the
repo root — see the root `docker-compose.yml` / `.env.example`).

```
cp ../.env.example ../.env   # adjust if you changed docker-compose.yml's credentials
npm install
```

## Build the database

The app assumes a pre-built DB and fails loudly without one. Out of season the
DB is built from the committed 10-weight fixture (last season's full container —
the launch backfill corpus). Requires Go and `$DATABASE_URL` set:

```
npm run db:build
```

or by hand, from `pipeline/`:

```
go run ./cmd/migrate -dir ../db/migrations
go run ./cmd/seed    -dir ../db/seed
go run ./cmd/scrape  -fixture internal/scraper/testdata/ranking_container_14300895_10weights.json
go run ./cmd/resolve
```

(all four default to `-db=$DATABASE_URL` when `-db` is omitted)

Expected result: 220 snapshots, 7080 entries, 519 wrestlers, and one logged
non-fatal tie anomaly (197, 2026-03-27 — see `docs/schema.md` §7).

## Configuration

| Env | Default | Meaning |
| --- | --- | --- |
| `DATABASE_URL` | *(required)* | Postgres connection string, shared with the Go pipeline — see `.env.example` |

## Commands

```
npm run dev      # dev server on :3000
npm test         # vitest — unit + query tests (needs $TEST_DATABASE_URL); the
                 # integration suite against the full rankings DB is skipped
                 # unless $DATABASE_URL points at a built one
npm run build    # production build to .output/
node .output/server/index.mjs   # run the production server
```

## Routes

- `/` — dashboard: latest edition for all ten weights; client-side weight
  filter, wrestler/school search, column sort (including movement).
- `/[weight]` — one weight's latest edition, SSR; `?date=YYYY-MM-DD` shows any
  historical edition (week selector).
- `/api/rankings`, `/api/rankings/[weight]?date=`,
  `/api/rankings/[weight]/series` — the Nitro routes behind those pages (the
  last powers the season-trajectory bump chart; `?sel=1,2` on the weight page
  shares a pinned selection).

## Data rules honored here (see `docs/schema.md`)

- Movement is **derived, never stored**: LAG over `published_date` per
  source/weight/season/wrestler (§5), ported from the pipeline's
  `MovementForWeight`. `prevRank` reaches back to the wrestler's last
  appearance *at that weight* — a mid-season weight change shows as movement
  from their last edition at the same weight, not Flo's cross-weight
  annotation.
- Rank is **non-unique** (§7): ties render verbatim, ordered by
  `rank, raw_source_string`.
- Display uses the point-in-time `raw_school` / `raw_grade` from each entry,
  never the canonical wrestler's current school.
- "Week N" is a display label derived from the edition's position in the
  season's dates (§4).
