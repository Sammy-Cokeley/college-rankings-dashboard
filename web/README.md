# web/ — Nuxt 3 front end

Fan-facing SSR app for the rankings board. Reads the pipeline-built SQLite DB
directly via Nitro server routes (no separate API layer in v0) and **never
writes** — the pipeline is the single weekly writer; WAL mode keeps reads and
the writer out of each other's way.

## Setup

Requires Node 24 LTS (better-sqlite3 ships prebuilt binaries for win32-x64 and
linux-arm64, so no build toolchain is needed on the dev box or the Pi).

```
npm install
```

## Build the database

The app assumes a pre-built DB and fails loudly without one. Out of season the
DB is built from the committed 10-weight fixture (last season's full container —
the launch backfill corpus). Requires Go:

```
npm run db:build
```

or by hand, from `pipeline/`:

```
go run ./cmd/migrate -db rankings.db -dir ../db/migrations
go run ./cmd/seed    -db rankings.db -dir ../db/seed
go run ./cmd/scrape  -db rankings.db -fixture internal/scraper/testdata/ranking_container_14300895_10weights.json
go run ./cmd/resolve -db rankings.db
```

Expected result: 220 snapshots, 7080 entries, 519 wrestlers, and one logged
non-fatal tie anomaly (197, 2026-03-27 — see `docs/schema.md` §7).

## Configuration

| Env | Default | Meaning |
| --- | --- | --- |
| `NUXT_DB_PATH` | `../pipeline/rankings.db` | SQLite file, resolved against the server process cwd |

## Commands

```
npm run dev      # dev server on :3000
npm test         # vitest — unit + query tests; the integration suite
                 # against the full rankings.db is skipped if it isn't built
npm run build    # production build to .output/
node .output/server/index.mjs   # run the production server
```

## Routes

- `/` — dashboard: latest edition for all ten weights; client-side weight
  filter, wrestler/school search, column sort (including movement).
- `/[weight]` — one weight's latest edition, SSR; `?date=YYYY-MM-DD` shows any
  historical edition (week selector).
- `/api/rankings`, `/api/rankings/[weight]?date=` — the Nitro routes behind
  those pages.

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
