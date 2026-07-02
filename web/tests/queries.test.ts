import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import Database from 'better-sqlite3'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import {
  editionEntries,
  getSeason,
  getSourceId,
  latestDate,
  listDates,
  type Db,
} from '../server/utils/queries'

const migration = resolve(
  dirname(fileURLToPath(import.meta.url)),
  '../../db/migrations/0001_init.sql',
)

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
let db: InstanceType<typeof Database>

const SRC = 1
const W = 125
const SEASON = 2026
const [d1, d2, d3, d4] = ['2026-01-06', '2026-01-13', '2026-01-20', '2026-01-27']

beforeAll(() => {
  db = new Database(':memory:')
  db.exec(readFileSync(migration, 'utf8'))
  db.exec(`
    INSERT INTO sources (id, name, type) VALUES (1, 'FloWrestling', 'editorial');
    INSERT INTO wrestlers (id, full_name) VALUES
      (1, 'Arn'), (2, 'Boe'), (3, 'Cox'), (4, 'Dye'), (5, 'Eck'), (6, 'Fox');
  `)

  const snap = db.prepare(
    `INSERT INTO snapshots (source_id, weight_class, season, published_date, captured_at)
     VALUES (?, ?, ?, ?, ?)`,
  )
  const entry = db.prepare(
    `INSERT INTO ranking_entries (snapshot_id, wrestler_id, rank, raw_source_string, raw_school, raw_grade)
     VALUES (?, ?, ?, ?, ?, ?)`,
  )

  const editions: Record<string, Array<[number | null, number, string, string | null, string | null]>> = {
    [d1]: [
      [1, 1, 'Arn', 'Iowa', 'SR'],
      [2, 2, 'Boe', 'Penn State', 'JR'],
      [3, 3, 'Cox', 'Cornell', 'SO'],
      [null, 9, 'Mystery Guy', null, null],
    ],
    [d2]: [
      [2, 1, 'Boe', 'Penn State', 'JR'],
      [1, 2, 'Arn', 'Iowa', 'SR'],
      [3, 3, 'Cox', 'Cornell', 'SO'],
      [4, 4, 'Dye', 'Ohio State', 'FR'],
      [null, 9, 'Mystery Gal', null, null],
    ],
    [d3]: [
      [1, 1, 'Arn', 'Iowa', 'SR'],
      [2, 2, 'Boe', 'Penn State', 'JR'],
      [4, 3, 'Dye', 'Ohio State', 'FR'],
    ],
    [d4]: [
      [1, 1, 'Arn', 'Iowa', 'SR'],
      [2, 2, 'Boe', 'Penn State', 'JR'],
      [4, 3, 'Dye', 'Ohio State', 'FR'],
      [3, 5, 'Cox', 'Cornell', 'SO'],
      [6, 6, 'Fox', 'Minnesota', 'SR'],
      [5, 6, 'Eck', 'Michigan', 'JR'],
    ],
  }

  for (const [date, rows] of Object.entries(editions)) {
    const snapId = snap.run(SRC, W, SEASON, date, `${date}T12:00:00Z`).lastInsertRowid
    for (const [wrestlerId, rank, name, school, grade] of rows) {
      entry.run(snapId, wrestlerId, rank, name, school, grade)
    }
  }

  // A second weight class in the same season must not bleed into 125's dates
  // or movement.
  const otherSnap = snap.run(SRC, 285, SEASON, d2, `${d2}T12:00:00Z`).lastInsertRowid
  entry.run(otherSnap, null, 1, 'Big Fella', 'Oklahoma State', 'SR')
})

afterAll(() => {
  db?.close()
})

describe('getSeason / getSourceId', () => {
  it('returns the max season present', () => {
    expect(getSeason(db as Db)).toBe(SEASON)
  })

  it('resolves the source by name and returns null for unknown sources', () => {
    expect(getSourceId(db as Db, 'FloWrestling')).toBe(SRC)
    expect(getSourceId(db as Db, 'InterMat')).toBeNull()
  })
})

describe('listDates', () => {
  it('returns ascending dates with 1-based week numbers, scoped to the weight', () => {
    expect(listDates(db as Db, SRC, W, SEASON)).toEqual([
      { date: d1, week: 1 },
      { date: d2, week: 2 },
      { date: d3, week: 3 },
      { date: d4, week: 4 },
    ])
    expect(listDates(db as Db, SRC, 285, SEASON)).toEqual([{ date: d2, week: 1 }])
  })

  it('returns empty for a weight with no snapshots', () => {
    expect(listDates(db as Db, SRC, 157, SEASON)).toEqual([])
  })
})

describe('latestDate', () => {
  it('returns the max published_date for the weight', () => {
    expect(latestDate(db as Db, SRC, W, SEASON)).toBe(d4)
    expect(latestDate(db as Db, SRC, 157, SEASON)).toBeNull()
  })
})

describe('editionEntries', () => {
  it('marks every entry as a first appearance in the opening edition', () => {
    const rows = editionEntries(db as Db, SRC, W, SEASON, d1)
    expect(rows.map((r) => [r.name, r.rank, r.prevRank])).toEqual([
      ['Arn', 1, null],
      ['Boe', 2, null],
      ['Cox', 3, null],
      ['Mystery Guy', 9, null],
    ])
  })

  it('derives movement from the previous edition, keyed by wrestler', () => {
    const rows = editionEntries(db as Db, SRC, W, SEASON, d2)
    const byName = Object.fromEntries(rows.map((r) => [r.name, r]))
    expect(byName['Boe']).toMatchObject({ rank: 1, prevRank: 2 }) // rose
    expect(byName['Arn']).toMatchObject({ rank: 2, prevRank: 1 }) // fell
    expect(byName['Cox']).toMatchObject({ rank: 3, prevRank: 3 }) // held
    expect(byName['Dye']).toMatchObject({ rank: 4, prevRank: null }) // new
  })

  it('never fabricates movement for unresolved (NULL wrestler_id) entries', () => {
    // Mystery Gal (d2) sits at the same rank Mystery Guy held in d1. Without
    // the NULL guard, SQLite's LAG would group both into one partition and
    // report prevRank 9 for Gal.
    const rows = editionEntries(db as Db, SRC, W, SEASON, d2)
    const gal = rows.find((r) => r.name === 'Mystery Gal')
    expect(gal).toMatchObject({ rank: 9, prevRank: null })
  })

  it('reaches back past a skipped week to the last appearance', () => {
    const rows = editionEntries(db as Db, SRC, W, SEASON, d4)
    const cox = rows.find((r) => r.name === 'Cox')
    expect(cox).toMatchObject({ rank: 5, prevRank: 3 }) // last seen d2 at 3
  })

  it('keeps tied ranks and orders them by raw_source_string', () => {
    const rows = editionEntries(db as Db, SRC, W, SEASON, d4)
    const tied = rows.filter((r) => r.rank === 6)
    expect(tied.map((r) => r.name)).toEqual(['Eck', 'Fox'])
  })

  it('carries raw_school and raw_grade through verbatim', () => {
    const rows = editionEntries(db as Db, SRC, W, SEASON, d1)
    expect(rows[0]).toMatchObject({ name: 'Arn', school: 'Iowa', grade: 'SR' })
    expect(rows[3]).toMatchObject({ name: 'Mystery Guy', school: null, grade: null })
  })

  it('returns empty for a date with no snapshot', () => {
    expect(editionEntries(db as Db, SRC, W, SEASON, '1999-01-01')).toEqual([])
  })
})
