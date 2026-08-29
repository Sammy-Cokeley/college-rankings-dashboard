import { afterAll, describe, expect, it } from 'vitest'
import postgres from 'postgres'
import {
  editionEntries,
  getSeason,
  getSourceId,
  latestDate,
  listDates,
  seasonSeries,
  type Db,
} from '../server/utils/queries'
import { seriesSegments } from '../utils/chart'

// Runs against the real pipeline-built DB (web/README.md: npm run db:build)
// and asserts the known facts from docs/sources/flowrestling-validation.md.
// Skipped when DATABASE_URL isn't set, or when the expected backfill corpus
// isn't present there (e.g. a fresh/empty dev database).
//
// The corpus check runs at TOP LEVEL with a real `await`, not inside
// beforeAll: describe.skipIf's condition is evaluated synchronously during
// test collection, before any beforeAll has had a chance to run — an
// async-populated `hasCorpus` read there would always still be its initial
// `false`, silently skipping this whole suite unconditionally regardless of
// DB state. Top-level await in an ESM test file genuinely blocks collection
// until the check resolves, which is what skipIf actually needs.
const dbUrl = process.env.DATABASE_URL

let db: Db | undefined
let hasCorpus = false

if (dbUrl) {
  db = postgres(dbUrl)
  try {
    const src = await getSourceId(db, 'FloWrestling')
    const season = src === null ? null : await getSeason(db, src)
    if (season !== null && src !== null) {
      const dates = await listDates(db, src, 125, season)
      hasCorpus = dates.length >= 22
    }
  } catch {
    hasCorpus = false
  }
}

afterAll(async () => {
  await db?.end()
})

describe.skipIf(!dbUrl || !hasCorpus)('pipeline-built rankings DB', () => {
  it('holds the full backfill corpus: 220 editions, 22 per weight', async () => {
    const src = (await getSourceId(db!, 'FloWrestling'))!
    const season = (await getSeason(db!, src))!
    expect(season).toBe(2026)
    for (const weight of [125, 133, 141, 149, 157, 165, 174, 184, 197, 285]) {
      expect(await listDates(db!, src, weight, season)).toHaveLength(22)
    }
  })

  it('surfaces the published 197 tie as two rank-21 rows in stable order', async () => {
    const src = (await getSourceId(db!, 'FloWrestling'))!
    const rows = await editionEntries(db!, src, 197, 2026, '2026-03-27')
    const tied = rows.filter((r) => r.rank === 21)
    expect(tied.map((r) => [r.name, r.school])).toEqual([
      ['Andrew Reall', 'Brown'],
      ['Dillon Bechtold', 'Bucknell'],
    ])
  })

  it('stays per-weight across a mid-season weight change', async () => {
    // Dylan Evans left 157 after 2025-10-29 (rank 21), wrestled 165 through
    // December, and returned to 157 on 2026-01-05. Flo's "Previous" column
    // annotates that return as "24 (165)" — his last rank at the OTHER weight.
    // Our per-weight model deliberately reaches back to his last 157
    // appearance instead: prevRank 21 (validation doc: the single known
    // divergence from Flo's own column, 6420/6421 agreement otherwise).
    const src = (await getSourceId(db!, 'FloWrestling'))!
    const rows = await editionEntries(db!, src, 157, 2026, '2026-01-05')
    const evans = rows.find((r) => r.name.includes('Dylan Evans'))
    expect(evans).toMatchObject({ rank: 16, prevRank: 21 })
  })

  it('builds a series for every resolved wrestler ranked at 157', async () => {
    const src = (await getSourceId(db!, 'FloWrestling'))!
    const series = await seasonSeries(db!, src, 157, 2026)
    const rows = await db!<{ n: string }[]>`
      SELECT COUNT(DISTINCT e.wrestler_id) AS n
      FROM ranking_entries e JOIN snapshots s ON s.id = e.snapshot_id
      WHERE s.source_id = ${src} AND s.weight_class = 157 AND s.season = 2026
        AND e.wrestler_id IS NOT NULL`
    expect(series.length).toBe(Number(rows[0]!.n))
    expect(series.length).toBeGreaterThanOrEqual(33)
  })

  it("renders Dylan Evans' 165 stint as a real gap in his 157 line", async () => {
    const src = (await getSourceId(db!, 'FloWrestling'))!
    const series = await seasonSeries(db!, src, 157, 2026)
    const evans = series.find((s) => s.name === 'Dylan Evans')!
    expect(evans.points).toHaveLength(14) // 22 editions minus his weeks at 165
    expect(seriesSegments(evans.points).length).toBeGreaterThan(1)
  })

  it('has a latest edition with a full table for every weight', async () => {
    const src = (await getSourceId(db!, 'FloWrestling'))!
    for (const weight of [125, 157, 285]) {
      const date = (await latestDate(db!, src, weight, 2026))!
      const rows = await editionEntries(db!, src, weight, 2026, date)
      expect(rows.length).toBeGreaterThanOrEqual(20)
      expect(rows[0]!.rank).toBe(1)
    }
  })
})
