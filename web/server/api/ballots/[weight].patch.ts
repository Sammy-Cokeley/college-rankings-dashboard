import { z } from 'zod'
import { isWeightClass } from '../../../utils/weights'
import { useDb } from '../../utils/db'
import { getRosterSeason, InvalidWrestlerError, saveBallot } from '../../utils/ballot-queries'
import { checkRateLimit } from '../../utils/rate-limit'

const bodySchema = z.object({
  // Ordered: index 0 = rank 1. Partial ballots are fine (schema.md §11 /
  // implementation plan §4) — no minimum length.
  wrestlerIds: z.array(z.number().int().positive()).max(33),
})

// PATCH /api/ballots/[weight] — replaces the current user's whole ballot at
// this weight with the given ordered wrestler list. Whole-list replace, not
// an incremental diff: it's what the autosave-the-whole-current-state UI
// model actually needs, and it's simpler than reconciling partial edits.
export default defineEventHandler(async (event) => {
  const weight = Number(getRouterParam(event, 'weight'))
  if (!isWeightClass(weight)) {
    throw createError({ statusCode: 404, statusMessage: 'Unknown weight class' })
  }

  const { user } = await requireUserSession(event)

  if (!checkRateLimit(`ballot_write:${user.id}`, 30, 60 * 1000)) {
    throw createError({ statusCode: 429, statusMessage: 'Too many ballot updates. Slow down.' })
  }

  const parsed = bodySchema.safeParse(await readBody(event))
  if (!parsed.success) {
    throw createError({ statusCode: 400, statusMessage: parsed.error.issues[0]?.message ?? 'Invalid request' })
  }
  const { wrestlerIds } = parsed.data
  if (new Set(wrestlerIds).size !== wrestlerIds.length) {
    throw createError({ statusCode: 400, statusMessage: 'A wrestler cannot appear twice on the same ballot' })
  }

  const db = useDb()
  const season = await getRosterSeason(db)
  if (season === null) {
    throw createError({ statusCode: 503, statusMessage: 'No roster data ingested yet' })
  }

  try {
    await saveBallot(db, user.id, weight, season, wrestlerIds)
  } catch (error: unknown) {
    if (error instanceof InvalidWrestlerError) {
      throw createError({ statusCode: 400, statusMessage: 'One or more wrestlers are not in the current roster pool' })
    }
    throw error
  }

  return { ok: true }
})
