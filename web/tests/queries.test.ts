import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import {
  editionEntries,
  getSeason,
  getSourceId,
  latestDate,
  listDates,
  seasonSeries,
  type Db,
} from '../server/utils/queries'
import { createTestDb } from './helpers/pg-test-db'

// Controlled corpus exercising every movement/tie/ordering rule the query
// layer must honor. Weight 125, season 2026, four editions d1..d4:
//
//   date  Arn  Boe  Cox  Dye  unresolved
//   d1     1    2    3    -   "Mystery Guy" rank 9
//   d2     2    1    3    4   "Mystery Gal" rank 9  (same rank as d1's — the
//                              NULL-partition trap: without the CASE guard,
//                              LAG would hand Gal a bogus prev_rank of 9)
//   d3     1    2    -    3   (Cox absent this week)
//   d4     1    2    5    3   Cox returns: prev_rank must reach back to d2 (3)
//
// d4 also publishes a tie: Eck and Fox both at rank 6 (entered Fox-first to
// prove ORDER BY raw_source_string, not insertion order).
let db: Db
let cleanup: () => Promise<void>

const W = 125
const SEASON = 2026
const [d1, d2, d3, d4] = ['2026-01-06', '2026-01-13', '2026-01-20', '2026-01-27']
let SRC: number

beforeAll(async () => {
  ;({ db, cleanup } = await createTestDb())

  await db`INSERT INTO sources (name, type) VALUES ('FloWrestling', 'editorial')`
  SRC = (await getSourceId(db, 'FloWrestling'))!

  const wrestlerIds = new Map<string, number>()
  for (const name of ['Arn', 'Boe', 'Cox', 'Dye', 'Eck', 'Fox']) {
    const rows = await db<{ id: number }[]>`
      INSERT INTO wrestlers (full_name) VALUES (${name}) RETURNING id`
    wrestlerIds.set(name, rows[0]!.id)
  }

  const insertSnap = async (date: string, weight: number) => {
    const rows = await db<{ id: number }[]>`
      INSERT INTO snapshots (source_id, weight_class, season, published_date, captured_at)
      VALUES (${SRC}, ${weight}, ${SEASON}, ${date}, ${date + 'T12:00:00Z'}) RETURNING id`
    return rows[0]!.id
  }
  const insertEntry = async (
    snapId: number,
    wrestlerName: string | null,
    rank: number,
    name: string,
    school: string | null,
    grade: string | null,
  ) => {
    const wrestlerId = wrestlerName ? wrestlerIds.get(wrestlerName)! : null
    await db`
      INSERT INTO ranking_entries (snapshot_id, wrestler_id, rank, raw_source_string, raw_school, raw_grade)
      VALUES (${snapId}, ${wrestlerId}, ${rank}, ${name}, ${school}, ${grade})`
  }

  const editions: Record<string, Array<[string | null, number, string, string | null, string | null]>> = {
    [d1]: [
      ['Arn', 1, 'Arn', 'Iowa', 'SR'],
      ['Boe', 2, 'Boe', 'Penn State', 'JR'],
      ['Cox', 3, 'Cox', 'Cornell', 'SO'],
      [null, 9, 'Mystery Guy', null, null],
    ],
    [d2]: [
      ['Boe', 1, 'Boe', 'Penn State', 'JR'],
      ['Arn', 2, 'Arn', 'Iowa', 'SR'],
      ['Cox', 3, 'Cox', 'Cornell', 'SO'],
      ['Dye', 4, 'Dye', 'Ohio State', 'FR'],
      [null, 9, 'Mystery Gal', null, null],
    ],
    [d3]: [
      ['Arn', 1, 'Arn', 'Iowa', 'SR'],
      ['Boe', 2, 'Boe', 'Penn State', 'JR'],
      ['Dye', 3, 'Dye', 'Ohio State', 'FR'],
    ],
    [d4]: [
      ['Arn', 1, 'Arn', 'Iowa', 'SR'],
      ['Boe', 2, 'Boe', 'Penn State', 'JR'],
      // School string drifts in the final edition: a series must surface the
      // latest raw spelling, while each entry keeps its own verbatim.
      ['Dye', 3, 'Dye', 'Ohio St.', 'FR'],
      ['Cox', 5, 'Cox', 'Cornell', 'SO'],
      ['Fox', 6, 'Fox', 'Minnesota', 'SR'],
      ['Eck', 6, 'Eck', 'Michigan', 'JR'],
    ],
  }

  for (const [date, rows] of Object.entries(editions)) {
    const snapId = await insertSnap(date, W)
    for (const [wrestlerName, rank, name, school, grade] of rows) {
      await insertEntry(snapId, wrestlerName, rank, name, school, grade)
    }
  }

  // A second weight class in the same season must not bleed into 125's dates
  // or movement.
  const otherSnap = await insertSnap(d2, 285)
  await insertEntry(otherSnap, null, 1, 'Big Fella', 'Oklahoma State', 'SR')
})

afterAll(async () => {
  await cleanup?.()
})

describe('getSeason / getSourceId', () => {
  it('returns the max season present for that source', async () => {
    expect(await getSeason(db, SRC)).toBe(SEASON)
  })

  it('is scoped per source, not global — a newer season on another source must not leak in', async () => {
    // Regression coverage for the bug this scoping fixed: an unscoped
    // MAX(season) worked by accident when exactly one source existed (v0);
    // a second source with a LATER season (e.g. the Fan Poll's current
    // roster season vs FloWrestling's completed-backfill season, schema.md
    // §10) must never change what getSeason(db, SRC) reports for SRC.
    const [{ id: otherSourceId }] = await db<{ id: number }[]>`
      INSERT INTO sources (name, type) VALUES ('Fan Poll', 'poll') RETURNING id`
    await db`
      INSERT INTO snapshots (source_id, weight_class, season, published_date, captured_at)
      VALUES (${otherSourceId}, 125, ${SEASON + 1}, '2027-01-01', '2027-01-01T00:00:00Z')`

    expect(await getSeason(db, SRC)).toBe(SEASON)
    expect(await getSeason(db, otherSourceId)).toBe(SEASON + 1)
  })

  it('resolves the source by name and returns null for unknown sources', async () => {
    expect(await getSourceId(db, 'FloWrestling')).toBe(SRC)
    expect(await getSourceId(db, 'InterMat')).toBeNull()
  })
})

describe('listDates', () => {
  it('returns ascending dates with 1-based week numbers, scoped to the weight', async () => {
    expect(await listDates(db, SRC, W, SEASON)).toEqual([
      { date: d1, week: 1 },
      { date: d2, week: 2 },
      { date: d3, week: 3 },
      { date: d4, week: 4 },
    ])
    expect(await listDates(db, SRC, 285, SEASON)).toEqual([{ date: d2, week: 1 }])
  })

  it('returns empty for a weight with no snapshots', async () => {
    expect(await listDates(db, SRC, 157, SEASON)).toEqual([])
  })
})

describe('latestDate', () => {
  it('returns the max published_date for the weight', async () => {
    expect(await latestDate(db, SRC, W, SEASON)).toBe(d4)
    expect(await latestDate(db, SRC, 157, SEASON)).toBeNull()
  })
})

describe('editionEntries', () => {
  it('marks every entry as a first appearance in the opening edition', async () => {
    const rows = await editionEntries(db, SRC, W, SEASON, d1)
    expect(rows.map((r) => [r.name, r.rank, r.prevRank])).toEqual([
      ['Arn', 1, null],
      ['Boe', 2, null],
      ['Cox', 3, null],
      ['Mystery Guy', 9, null],
    ])
  })

  it('derives movement from the previous edition, keyed by wrestler', async () => {
    const rows = await editionEntries(db, SRC, W, SEASON, d2)
    const byName = Object.fromEntries(rows.map((r) => [r.name, r]))
    expect(byName['Boe']).toMatchObject({ rank: 1, prevRank: 2 }) // rose
    expect(byName['Arn']).toMatchObject({ rank: 2, prevRank: 1 }) // fell
    expect(byName['Cox']).toMatchObject({ rank: 3, prevRank: 3 }) // held
    expect(byName['Dye']).toMatchObject({ rank: 4, prevRank: null }) // new
  })

  it('never fabricates movement for unresolved (NULL wrestler_id) entries', async () => {
    // Mystery Gal (d2) sits at the same rank Mystery Guy held in d1. Without
    // the NULL guard, LAG would group both into one partition and report
    // prevRank 9 for Gal.
    const rows = await editionEntries(db, SRC, W, SEASON, d2)
    const gal = rows.find((r) => r.name === 'Mystery Gal')
    expect(gal).toMatchObject({ rank: 9, prevRank: null })
  })

  it('reaches back past a skipped week to the last appearance', async () => {
    const rows = await editionEntries(db, SRC, W, SEASON, d4)
    const cox = rows.find((r) => r.name === 'Cox')
    expect(cox).toMatchObject({ rank: 5, prevRank: 3 }) // last seen d2 at 3
  })

  it('keeps tied ranks and orders them by raw_source_string', async () => {
    const rows = await editionEntries(db, SRC, W, SEASON, d4)
    const tied = rows.filter((r) => r.rank === 6)
    expect(tied.map((r) => r.name)).toEqual(['Eck', 'Fox'])
  })

  it('carries raw_school and raw_grade through verbatim', async () => {
    const rows = await editionEntries(db, SRC, W, SEASON, d1)
    expect(rows[0]).toMatchObject({ name: 'Arn', school: 'Iowa', grade: 'SR' })
    expect(rows[3]).toMatchObject({ name: 'Mystery Guy', school: null, grade: null })
  })

  it('returns empty for a date with no snapshot', async () => {
    expect(await editionEntries(db, SRC, W, SEASON, '1999-01-01')).toEqual([])
  })

  it('carries the canonical wrestlerId (null when unresolved)', async () => {
    const rows = await editionEntries(db, SRC, W, SEASON, d1)
    const byName = Object.fromEntries(rows.map((r) => [r.name, r.wrestlerId]))
    expect(byName['Arn']).toBeTypeOf('number')
    expect(byName['Mystery Guy']).toBeNull()
  })
})

describe('seasonSeries', () => {
  it('builds one series per resolved wrestler, in first-appearance order', async () => {
    const series = await seasonSeries(db, SRC, W, SEASON)
    expect(series.map((s) => s.name)).toEqual(['Arn', 'Boe', 'Cox', 'Dye', 'Eck', 'Fox'])
  })

  it('maps ranks onto derived week numbers', async () => {
    const series = await seasonSeries(db, SRC, W, SEASON)
    const arn = series.find((s) => s.name === 'Arn')!
    expect(arn.points).toEqual([
      { week: 1, rank: 1 },
      { week: 2, rank: 2 },
      { week: 3, rank: 1 },
      { week: 4, rank: 1 },
    ])
  })

  it('leaves a gap (no point) for a skipped week — never interpolates', async () => {
    const series = await seasonSeries(db, SRC, W, SEASON)
    const cox = series.find((s) => s.name === 'Cox')!
    expect(cox.points).toEqual([
      { week: 1, rank: 3 },
      { week: 2, rank: 3 },
      { week: 4, rank: 5 },
    ])
  })

  it('keeps both wrestlers of a tied rank as separate series points', async () => {
    const series = await seasonSeries(db, SRC, W, SEASON)
    const eck = series.find((s) => s.name === 'Eck')!
    const fox = series.find((s) => s.name === 'Fox')!
    expect(eck.points).toEqual([{ week: 4, rank: 6 }])
    expect(fox.points).toEqual([{ week: 4, rank: 6 }])
  })

  it('excludes unresolved (NULL wrestler_id) entries — no identity, no line', async () => {
    const series = await seasonSeries(db, SRC, W, SEASON)
    expect(series.some((s) => s.name.startsWith('Mystery'))).toBe(false)
  })

  it('surfaces the latest raw name/school spelling on the series', async () => {
    const series = await seasonSeries(db, SRC, W, SEASON)
    const dye = series.find((s) => s.name === 'Dye')!
    expect(dye.school).toBe('Ohio St.') // drifted in d4; entries keep verbatim
  })

  it('returns empty for a weight with no snapshots', async () => {
    expect(await seasonSeries(db, SRC, 157, SEASON)).toEqual([])
  })
})
