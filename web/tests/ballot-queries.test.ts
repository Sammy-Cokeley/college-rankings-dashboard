import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import type { Db } from '../server/utils/queries'
import { getBallot, getRosterSeason, InvalidWrestlerError, saveBallot, searchWrestlers } from '../server/utils/ballot-queries'
import { createTestDb } from './helpers/pg-test-db'

let db: Db
let cleanup: () => Promise<void>

const SEASON = 2027
let userId: number
let iowaWrestler: number // roster-assigned 149, Iowa
let iowaWrestler133: number // roster-assigned 133, Iowa — off-weight ballot pick target
let unrostered: number // resolved wrestler, no current-season roster row

beforeAll(async () => {
  ;({ db, cleanup } = await createTestDb())

  const [school] = await db<{ id: number }[]>`
    INSERT INTO schools (name, division) VALUES ('Iowa', 'DI') RETURNING id`
  const schoolId = school!.id

  const mkWrestler = async (name: string) => {
    const [w] = await db<{ id: number }[]>`INSERT INTO wrestlers (full_name) VALUES (${name}) RETURNING id`
    return w!.id
  }
  iowaWrestler = await mkWrestler('Real Deal')
  iowaWrestler133 = await mkWrestler('Off Weight Guy')
  unrostered = await mkWrestler('No Longer Rostered')

  const mkRoster = async (wrestlerId: number, weight: number, name: string) =>
    db`
      INSERT INTO roster_entries (school_id, wrestler_id, season, weight_class, raw_name, raw_source_string, captured_at)
      VALUES (${schoolId}, ${wrestlerId}, ${SEASON}, ${weight}, ${name}, ${name}, ${new Date().toISOString()})`
  await mkRoster(iowaWrestler, 149, 'Real Deal')
  await mkRoster(iowaWrestler133, 133, 'Off Weight Guy')
  // unrostered deliberately gets no roster_entries row this season.

  const [user] = await db<{ id: number }[]>`
    INSERT INTO users (email, password_hash, created_at)
    VALUES ('ballot-test@example.com', 'x', ${new Date().toISOString()}) RETURNING id`
  userId = user!.id
})

afterAll(async () => {
  await cleanup?.()
})

describe('getRosterSeason', () => {
  it('returns the max season present in roster_entries', async () => {
    expect(await getRosterSeason(db)).toBe(SEASON)
  })
})

describe('searchWrestlers', () => {
  it('matches by name substring, case-insensitive', async () => {
    const results = await searchWrestlers(db, 149, 'real deal', SEASON)
    expect(results.map((r) => r.name)).toEqual(['Real Deal'])
  })

  it('surfaces wrestlers at the given weight first, but does not exclude others', async () => {
    const results = await searchWrestlers(db, 149, '', SEASON)
    const names = results.map((r) => r.name)
    expect(names).toContain('Real Deal')
    expect(names).toContain('Off Weight Guy') // roster-assigned 133, still returned
    expect(names.indexOf('Real Deal')).toBeLessThan(names.indexOf('Off Weight Guy'))
  })

  it('excludes wrestlers with no roster row this season', async () => {
    const results = await searchWrestlers(db, 149, 'No Longer Rostered', SEASON)
    expect(results).toEqual([])
  })
})

describe('getBallot / saveBallot', () => {
  it('returns an empty shell before any ballot exists', async () => {
    const ballot = await getBallot(db, userId, 157, SEASON)
    expect(ballot).toEqual({ weightClass: 157, season: SEASON, updatedAt: null, entries: [] })
  })

  it('saves an ordered ballot and getBallot reflects rank + school', async () => {
    await saveBallot(db, userId, 149, SEASON, [iowaWrestler133, iowaWrestler])
    const ballot = await getBallot(db, userId, 149, SEASON)
    expect(ballot.entries).toEqual([
      { rank: 1, wrestlerId: iowaWrestler133, name: 'Off Weight Guy', school: 'Iowa' },
      { rank: 2, wrestlerId: iowaWrestler, name: 'Real Deal', school: 'Iowa' },
    ])
    expect(ballot.updatedAt).not.toBeNull()
  })

  it('allows ranking a wrestler at a weight other than their roster-assigned one', async () => {
    // Off Weight Guy is roster-assigned 133 (see setup) but this ballot is
    // for 149 — schema.md §8: the ballot's own weight governs, not the
    // wrestler's roster listing.
    await saveBallot(db, userId, 149, SEASON, [iowaWrestler133])
    const ballot = await getBallot(db, userId, 149, SEASON)
    expect(ballot.entries).toEqual([
      { rank: 1, wrestlerId: iowaWrestler133, name: 'Off Weight Guy', school: 'Iowa' },
    ])
  })

  it('re-saving replaces the whole ballot, not merges', async () => {
    await saveBallot(db, userId, 149, SEASON, [iowaWrestler])
    const ballot = await getBallot(db, userId, 149, SEASON)
    expect(ballot.entries).toHaveLength(1)
    expect(ballot.entries[0]!.wrestlerId).toBe(iowaWrestler)
  })

  it('saving an empty list clears the ballot', async () => {
    await saveBallot(db, userId, 149, SEASON, [])
    const ballot = await getBallot(db, userId, 149, SEASON)
    expect(ballot.entries).toEqual([])
  })

  it('rejects a wrestler with no roster row this season', async () => {
    await expect(saveBallot(db, userId, 149, SEASON, [unrostered])).rejects.toBeInstanceOf(
      InvalidWrestlerError,
    )
  })

  it('is scoped per weight — saving 149 does not touch 157', async () => {
    await saveBallot(db, userId, 149, SEASON, [iowaWrestler])
    await saveBallot(db, userId, 157, SEASON, [iowaWrestler133])
    const b149 = await getBallot(db, userId, 149, SEASON)
    const b157 = await getBallot(db, userId, 157, SEASON)
    expect(b149.entries.map((e) => e.wrestlerId)).toEqual([iowaWrestler])
    expect(b157.entries.map((e) => e.wrestlerId)).toEqual([iowaWrestler133])
  })
})
