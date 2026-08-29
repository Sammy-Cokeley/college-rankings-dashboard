import type { Edition, RankingsOverview } from '../../../types/rankings'
import { WEIGHT_CLASSES } from '../../../utils/weights'
import { useDb } from '../../utils/db'
import { editionEntries, getSeason, getSourceId, listDates } from '../../utils/queries'
import { resolveSource } from '../../utils/source'

// GET /api/rankings?source=<slug> — the dashboard payload: the latest edition
// (entries + movement) for every weight class in the newest season, for one
// source (defaults to FloWrestling; see utils/sources.ts).
export default defineEventHandler(async (event): Promise<RankingsOverview> => {
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

  const weights: Edition[] = []
  for (const weight of WEIGHT_CLASSES) {
    const dates = await listDates(db, sourceId, weight, season)
    const latest = dates[dates.length - 1]
    if (!latest) continue
    weights.push({
      weight,
      season,
      date: latest.date,
      week: latest.week,
      entries: await editionEntries(db, sourceId, weight, season, latest.date),
    })
  }
  return { source: source.name, season, weights }
})
