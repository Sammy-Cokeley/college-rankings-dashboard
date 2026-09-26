import type { Db } from './queries'
import type { Ballot, BallotEntry, BallotSubmission, WrestlerOption } from '../../types/ballots'

// getRosterSeason returns the newest season present in roster_entries — NOT
// the same value as getSeason() (rankings), and not expected to be
// (schema.md §10). A ballot ranks the CURRENT roster pool, which is
// forward-looking relative to whatever rankings happen to be on display.
export async function getRosterSeason(db: Db): Promise<number | null> {
  const rows = await db<{ season: number | null }[]>`SELECT MAX(season) AS season FROM roster_entries`
  return rows[0]?.season ?? null
}

// searchWrestlers returns resolved roster wrestlers for the given season,
// optionally filtered by a name substring. Wrestlers whose roster-assigned
// weight matches `weight` sort first, but the search is never restricted to
// that weight — a user may deliberately add someone to a ballot at a weight
// other than their roster listing (schema.md §8).
export async function searchWrestlers(
  db: Db,
  weight: number,
  query: string,
  season: number,
): Promise<WrestlerOption[]> {
  const needle = query.trim()
  const rows = await db<WrestlerOption[]>`
    SELECT w.id AS "wrestlerId", w.full_name AS name, s.name AS school,
           re.weight_class AS "weightClass"
    FROM roster_entries re
    JOIN wrestlers w ON w.id = re.wrestler_id
    JOIN schools s ON s.id = re.school_id
    WHERE re.season = ${season}
      AND (${needle}::text = '' OR w.full_name ILIKE ${'%' + needle + '%'})
    ORDER BY (re.weight_class = ${weight}) DESC, w.full_name
    LIMIT 50`
  return rows
}

// getBallot returns a user's current ballot at one weight, or an empty shell
// (updatedAt: null, entries: []) if they haven't started one — no row is
// created just by looking.
export async function getBallot(
  db: Db,
  userId: number,
  weight: number,
  season: number,
): Promise<Ballot> {
  // School comes from this season's roster_entries, not wrestlers.current_
  // school_id — that column is never populated anywhere in the pipeline
  // today (resolve/wrestlers.go's InsertWrestler leaves it NULL by design;
  // v0 school canonicalization for rankings was out of scope). roster_entries
  // is the only place a real, current school actually lives.
  const rows = await db<
    Array<{ updatedAt: string; rank: number; wrestlerId: number; name: string; school: string | null }>
  >`
    SELECT b.updated_at AS "updatedAt", be.rank, be.wrestler_id AS "wrestlerId",
           w.full_name AS name, s.name AS school
    FROM ballots b
    JOIN ballot_entries be ON be.ballot_id = b.id
    JOIN wrestlers w ON w.id = be.wrestler_id
    LEFT JOIN roster_entries re ON re.wrestler_id = w.id AND re.season = ${season}
    LEFT JOIN schools s ON s.id = re.school_id
    WHERE b.user_id = ${userId} AND b.weight_class = ${weight} AND b.season = ${season}
    ORDER BY be.rank`

  const entries: BallotEntry[] = rows.map((r) => ({
    rank: r.rank,
    wrestlerId: r.wrestlerId,
    name: r.name,
    school: r.school,
  }))
  return {
    weightClass: weight,
    season,
    updatedAt: rows[0]?.updatedAt ?? null,
    entries,
  }
}

export class InvalidWrestlerError extends Error {
  constructor(public wrestlerIds: number[]) {
    super(`wrestler_id(s) not in this season's roster: ${wrestlerIds.join(', ')}`)
  }
}

export class EmptyBallotError extends Error {
  constructor() {
    super('cannot submit a ballot with no entries')
  }
}

// saveBallot replaces a user's entire ballot at one weight with the given
// ordered wrestler list (rank = array position + 1) — simpler and safer than
// diffing against the previous state, and matches the autosave-the-whole-
// current-state model the ballot builder UI uses. Every id is checked
// against the current season's roster_entries first (schema.md §11); the
// FOREIGN KEY alone only proves the id is SOME canonical wrestler, not a
// current roster member.
export async function saveBallot(
  db: Db,
  userId: number,
  weight: number,
  season: number,
  wrestlerIds: number[],
): Promise<void> {
  if (wrestlerIds.length > 0) {
    const valid = await db<{ wrestlerId: number }[]>`
      SELECT DISTINCT wrestler_id AS "wrestlerId"
      FROM roster_entries
      WHERE season = ${season} AND wrestler_id = ANY(${wrestlerIds})`
    const validIds = new Set(valid.map((r) => r.wrestlerId))
    const invalid = wrestlerIds.filter((id) => !validIds.has(id))
    if (invalid.length > 0) {
      throw new InvalidWrestlerError(invalid)
    }
  }

  const now = new Date().toISOString()
  await db.begin(async (sql) => {
    const [ballot] = await sql<{ id: number }[]>`
      INSERT INTO ballots (user_id, weight_class, season, updated_at)
      VALUES (${userId}, ${weight}, ${season}, ${now})
      ON CONFLICT (user_id, weight_class, season) DO UPDATE SET updated_at = ${now}
      RETURNING id`
    const ballotId = ballot!.id

    await sql`DELETE FROM ballot_entries WHERE ballot_id = ${ballotId}`
    if (wrestlerIds.length === 0) return

    const rows = wrestlerIds.map((wrestlerId, i) => ({
      ballot_id: ballotId,
      rank: i + 1,
      wrestler_id: wrestlerId,
    }))
    await sql`INSERT INTO ballot_entries ${sql(rows, 'ballot_id', 'rank', 'wrestler_id')}`
  })
}

// submitBallot snapshots a user's already-persisted ballot (whatever's
// currently saved via saveBallot/autosave) into ballot_submissions — the
// append-only history cmd/aggregate-poll and /profile's history page read
// from. No wrestlerIds parameter: it copies live ballot_entries rather than
// trusting a client-supplied list, so a submission always exactly matches
// what's actually saved, never something the client merely claims is saved.
export async function submitBallot(
  db: Db,
  userId: number,
  weight: number,
  season: number,
): Promise<void> {
  const now = new Date().toISOString()
  await db.begin(async (sql) => {
    const [ballot] = await sql<{ id: number }[]>`
      SELECT id FROM ballots
      WHERE user_id = ${userId} AND weight_class = ${weight} AND season = ${season}`
    const entries = ballot
      ? await sql<{ rank: number; wrestlerId: number }[]>`
          SELECT rank, wrestler_id AS "wrestlerId"
          FROM ballot_entries WHERE ballot_id = ${ballot.id} ORDER BY rank`
      : []
    if (entries.length === 0) {
      throw new EmptyBallotError()
    }

    const [submission] = await sql<{ id: number }[]>`
      INSERT INTO ballot_submissions (user_id, weight_class, season, submitted_at)
      VALUES (${userId}, ${weight}, ${season}, ${now})
      RETURNING id`
    const submissionId = submission!.id

    const rows = entries.map((e) => ({
      submission_id: submissionId,
      rank: e.rank,
      wrestler_id: e.wrestlerId,
    }))
    await sql`INSERT INTO ballot_submission_entries ${sql(rows, 'submission_id', 'rank', 'wrestler_id')}`
  })
}

// getBallotHistory returns a user's full submission history across every
// weight class, newest first — what /profile's "Previous ballots" list
// reads. School is joined per-submission's own `season` (not the current
// roster season) so a past submission shows the school as it was at the
// time, matching getBallot's join pattern.
export async function getBallotHistory(db: Db, userId: number): Promise<BallotSubmission[]> {
  const rows = await db<
    Array<{
      id: number
      weightClass: number
      submittedAt: string
      rank: number
      wrestlerId: number
      name: string
      school: string | null
    }>
  >`
    SELECT bs.id, bs.weight_class AS "weightClass", bs.submitted_at AS "submittedAt",
           bse.rank, bse.wrestler_id AS "wrestlerId", w.full_name AS name, s.name AS school
    FROM ballot_submissions bs
    JOIN ballot_submission_entries bse ON bse.submission_id = bs.id
    JOIN wrestlers w ON w.id = bse.wrestler_id
    LEFT JOIN roster_entries re ON re.wrestler_id = w.id AND re.season = bs.season
    LEFT JOIN schools s ON s.id = re.school_id
    WHERE bs.user_id = ${userId}
    ORDER BY bs.submitted_at DESC, bs.id DESC, bse.rank`

  const bySubmission = new Map<number, BallotSubmission>()
  for (const r of rows) {
    let submission = bySubmission.get(r.id)
    if (!submission) {
      submission = { id: r.id, weightClass: r.weightClass, submittedAt: r.submittedAt, entries: [] }
      bySubmission.set(r.id, submission)
    }
    submission.entries.push({ rank: r.rank, wrestlerId: r.wrestlerId, name: r.name, school: r.school })
  }
  return [...bySubmission.values()]
}
