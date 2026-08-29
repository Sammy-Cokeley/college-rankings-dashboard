import { isWeightClass } from '../../../utils/weights'
import { useDb } from '../../utils/db'
import { getBallot, getRosterSeason } from '../../utils/ballot-queries'

// GET /api/ballots/[weight] — the current user's ballot at one weight.
// Auth-guarded: a ballot is per-user, there's nothing to read without a
// session (guests keep their in-progress ballot client-side; see
// pages/ballot/[weight].vue).
export default defineEventHandler(async (event) => {
  const weight = Number(getRouterParam(event, 'weight'))
  if (!isWeightClass(weight)) {
    throw createError({ statusCode: 404, statusMessage: 'Unknown weight class' })
  }

  const { user } = await requireUserSession(event)

  const db = useDb()
  const season = await getRosterSeason(db)
  if (season === null) {
    throw createError({ statusCode: 503, statusMessage: 'No roster data ingested yet' })
  }

  return getBallot(db, user.id, weight, season)
})
