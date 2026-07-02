import type { Edition, RankingsOverview } from '../../../types/rankings'
import { WEIGHT_CLASSES } from '../../../utils/weights'
import { useDb } from '../../utils/db'
import { editionEntries, getSeason, getSourceId, listDates } from '../../utils/queries'
import { SOURCE_NAME } from '../../utils/source'

// GET /api/rankings — the dashboard payload: the latest edition (entries +
// movement) for every weight class in the newest season.
export default defineEventHandler((): RankingsOverview => {
  const db = useDb()
  const season = getSeason(db)
  const sourceId = getSourceId(db, SOURCE_NAME)
  if (season === null || sourceId === null) {
    throw createError({ statusCode: 503, statusMessage: 'No ranking data ingested yet' })
  }

  const weights: Edition[] = []
  for (const weight of WEIGHT_CLASSES) {
    const dates = listDates(db, sourceId, weight, season)
    const latest = dates[dates.length - 1]
    if (!latest) continue
    weights.push({
      weight,
      season,
      date: latest.date,
      week: latest.week,
      entries: editionEntries(db, sourceId, weight, season, latest.date),
    })
  }
  return { source: SOURCE_NAME, season, weights }
})
