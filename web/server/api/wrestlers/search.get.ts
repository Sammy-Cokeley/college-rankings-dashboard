import { z } from 'zod'
import { isWeightClass } from '../../../utils/weights'
import { useDb } from '../../utils/db'
import { getRosterSeason, searchWrestlers } from '../../utils/ballot-queries'

const querySchema = z.object({
  weight: z.coerce.number().refine(isWeightClass, 'Unknown weight class'),
  q: z.string().max(100).optional().default(''),
})

// GET /api/wrestlers/search?weight=149&q=... — public read (guests need this
// too, to build a ballot before signing up). Not restricted to the given
// weight — see WrestlerOption's doc comment / schema.md §8.
export default defineEventHandler(async (event) => {
  const parsed = querySchema.safeParse(getQuery(event))
  if (!parsed.success) {
    throw createError({ statusCode: 400, statusMessage: parsed.error.issues[0]?.message ?? 'Invalid request' })
  }

  const db = useDb()
  const season = await getRosterSeason(db)
  if (season === null) {
    throw createError({ statusCode: 503, statusMessage: 'No roster data ingested yet' })
  }

  return searchWrestlers(db, parsed.data.weight, parsed.data.q, season)
})
