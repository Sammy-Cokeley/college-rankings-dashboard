import type { SeasonSeries } from '../../../../types/rankings'
import { isWeightClass } from '../../../../utils/weights'
import { useDb } from '../../../utils/db'
import { getSeason, getSourceId, listDates, seasonSeries } from '../../../utils/queries'
import { resolveSource } from '../../../utils/source'

// GET /api/rankings/[weight]/series?source=<slug> — the bump-chart payload:
// every resolved wrestler's rank-over-week line for the newest season, plus
// the week spine.
export default defineEventHandler(async (event): Promise<SeasonSeries> => {
  const weight = Number(getRouterParam(event, 'weight'))
  if (!isWeightClass(weight)) {
    throw createError({ statusCode: 404, statusMessage: 'Unknown weight class' })
  }

  const source = resolveSource(getQuery(event).source as string | string[] | undefined)
  if (source === null) {
    throw createError({ statusCode: 404, statusMessage: 'Unknown source' })
  }

  const db = useDb()
  const sourceId = await getSourceId(db, source.name)
  const season = sourceId === null ? null : await getSeason(db, sourceId)
  if (season === null || sourceId === null) {
    throw createError({ statusCode: 503, statusMessage: 'No ranking data ingested yet' })
  }

  const weeks = await listDates(db, sourceId, weight, season)
  if (weeks.length === 0) {
    throw createError({ statusCode: 404, statusMessage: 'No editions for this weight class' })
  }

  return {
    source: source.name,
    weight,
    season,
    weeks,
    series: await seasonSeries(db, sourceId, weight, season),
  }
})
