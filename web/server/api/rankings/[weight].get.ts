import type { WeightRankings } from '../../../types/rankings'
import { isWeightClass } from '../../../utils/weights'
import { useDb } from '../../utils/db'
import { editionEntries, getSeason, getSourceId, listDates } from '../../utils/queries'
import { SOURCE_NAME } from '../../utils/source'

const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/

// GET /api/rankings/[weight]?date=YYYY-MM-DD — one weight's edition (latest by
// default) plus every edition date for the week selector. Params are validated
// here at the boundary; queries only ever see whitelisted/known values.
export default defineEventHandler((event): WeightRankings => {
  const weight = Number(getRouterParam(event, 'weight'))
  if (!isWeightClass(weight)) {
    throw createError({ statusCode: 404, statusMessage: 'Unknown weight class' })
  }

  const db = useDb()
  const season = getSeason(db)
  const sourceId = getSourceId(db, SOURCE_NAME)
  if (season === null || sourceId === null) {
    throw createError({ statusCode: 503, statusMessage: 'No ranking data ingested yet' })
  }

  const dates = listDates(db, sourceId, weight, season)
  if (dates.length === 0) {
    throw createError({ statusCode: 404, statusMessage: 'No editions for this weight class' })
  }

  const requested = getQuery(event).date
  let edition = dates[dates.length - 1]!
  if (requested !== undefined) {
    if (typeof requested !== 'string' || !ISO_DATE.test(requested)) {
      throw createError({ statusCode: 400, statusMessage: 'date must be YYYY-MM-DD' })
    }
    const match = dates.find((d) => d.date === requested)
    if (!match) {
      throw createError({ statusCode: 404, statusMessage: 'No edition published on that date' })
    }
    edition = match
  }

  return {
    source: SOURCE_NAME,
    edition: {
      weight,
      season,
      date: edition.date,
      week: edition.week,
      entries: editionEntries(db, sourceId, weight, season, edition.date),
    },
    dates,
  }
})
