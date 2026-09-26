import { isWeightClass } from '../../../../utils/weights'
import { useDb } from '../../../utils/db'
import { EmptyBallotError, getRosterSeason, submitBallot } from '../../../utils/ballot-queries'
import { checkRateLimit } from '../../../utils/rate-limit'

// POST /api/ballots/[weight]/submit — snapshots the current user's
// already-persisted ballot into ballot_submissions, the append-only history
// cmd/aggregate-poll reads from and /profile's history page displays. No
// request body: submitBallot copies whatever's already saved via autosave,
// so a submission always exactly matches what's actually on the ballot.
// Lower rate ceiling than the PATCH autosave endpoint (30/min) — this is a
// deliberate, infrequent action, not continuous autosave.
export default defineEventHandler(async (event) => {
  const weight = Number(getRouterParam(event, 'weight'))
  if (!isWeightClass(weight)) {
    throw createError({ statusCode: 404, statusMessage: 'Unknown weight class' })
  }

  const { user } = await requireUserSession(event)

  if (!checkRateLimit(`ballot_submit:${user.id}`, 10, 60 * 60 * 1000)) {
    throw createError({ statusCode: 429, statusMessage: 'Too many submissions. Slow down.' })
  }

  const db = useDb()
  const season = await getRosterSeason(db)
  if (season === null) {
    throw createError({ statusCode: 503, statusMessage: 'No roster data ingested yet' })
  }

  try {
    await submitBallot(db, user.id, weight, season)
  } catch (error: unknown) {
    if (error instanceof EmptyBallotError) {
      throw createError({ statusCode: 400, statusMessage: 'Add at least one wrestler before submitting' })
    }
    throw error
  }

  return { ok: true }
})
