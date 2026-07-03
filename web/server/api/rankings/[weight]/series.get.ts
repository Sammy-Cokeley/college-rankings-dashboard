import type { SeasonSeries } from '../../../../types/rankings'
import { isWeightClass } from '../../../../utils/weights'
import { useDb } from '../../../utils/db'
import { getSeason, getSourceId, listDates, seasonSeries } from '../../../utils/queries'
import { SOURCE_NAME } from '../../../utils/source'

// GET /api/rankings/[weight]/series — the bump-chart payload: every resolved
// wrestler's rank-over-week line for the newest season, plus the week spine.
export default defineEventHandler((event): SeasonSeries => {
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

  const weeks = listDates(db, sourceId, weight, season)
  if (weeks.length === 0) {
    throw createError({ statusCode: 404, statusMessage: 'No editions for this weight class' })
  }

  return {
    source: SOURCE_NAME,
    weight,
    season,
    weeks,
    series: seasonSeries(db, sourceId, weight, season),
  }
})
