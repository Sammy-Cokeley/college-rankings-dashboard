import type DatabaseConstructor from 'better-sqlite3'
import type { EditionDate, RankingRow, WrestlerSeries } from '../../types/rankings'

export type Db = InstanceType<typeof DatabaseConstructor>

// getSeason returns the newest season in the store, or null when empty.
// v0 is single-season; the newest one is what the site shows.
export function getSeason(db: Db): number | null {
  const row = db.prepare('SELECT MAX(season) AS season FROM snapshots').get() as {
    season: number | null
  }
  return row.season
}

// getSourceId resolves a source by its seeded canonical name.
export function getSourceId(db: Db, name: string): number | null {
  const row = db.prepare('SELECT id FROM sources WHERE name = ?').get(name) as
    | { id: number }
    | undefined
  return row?.id ?? null
}

// listDates returns every edition date for one source/weight/season, ascending,
// with the display week derived as the 1-based position (schema.md §4: the
// published_date is the time spine; week numbers are an app-level label).
export function listDates(db: Db, sourceId: number, weight: number, season: number): EditionDate[] {
  const rows = db
    .prepare(
      `SELECT DISTINCT published_date AS date
       FROM snapshots
       WHERE source_id = ? AND weight_class = ? AND season = ?
       ORDER BY published_date`,
    )
    .all(sourceId, weight, season) as Array<{ date: string }>
  return rows.map((r, i) => ({ date: r.date, week: i + 1 }))
}

// latestDate returns the newest edition date for one source/weight/season.
export function latestDate(
  db: Db,
  sourceId: number,
  weight: number,
  season: number,
): string | null {
  const row = db
    .prepare(
      `SELECT MAX(published_date) AS date
       FROM snapshots
       WHERE source_id = ? AND weight_class = ? AND season = ?`,
    )
    .get(sourceId, weight, season) as { date: string | null }
  return row.date
}

// editionEntries returns one edition's rows with each wrestler's previous rank
// attached. This is the pipeline's MovementForWeight query (schema.md §5;
// pipeline/internal/store/snapshots.go) computed over the whole season so LAG
// can see history, then filtered to the requested date. Two invariants carried
// over: rank is NOT unique (ties, §7), so order is rank then raw_source_string;
// and prev_rank reaches back to the wrestler's last appearance, not strictly
// last week — matching Flo's own "Previous" column semantics.
//
// One deliberate divergence from the Go port: unresolved entries (NULL
// wrestler_id) get a NULL prev_rank via CASE. SQLite's LAG lumps all NULLs
// into one partition, which would fabricate movement between unrelated
// unresolved wrestlers.
export function editionEntries(
  db: Db,
  sourceId: number,
  weight: number,
  season: number,
  date: string,
): RankingRow[] {
  return db
    .prepare(
      `WITH season_entries AS (
         SELECT s.published_date,
                e.rank,
                e.wrestler_id,
                e.raw_source_string,
                e.raw_school,
                e.raw_grade,
                CASE WHEN e.wrestler_id IS NULL THEN NULL ELSE
                  LAG(e.rank) OVER (
                    PARTITION BY s.source_id, s.weight_class, s.season, e.wrestler_id
                    ORDER BY s.published_date
                  )
                END AS prev_rank
         FROM ranking_entries e
         JOIN snapshots s ON s.id = e.snapshot_id
         WHERE s.source_id = ? AND s.weight_class = ? AND s.season = ?
       )
       SELECT rank,
              raw_source_string AS name,
              raw_school        AS school,
              raw_grade         AS grade,
              prev_rank         AS prevRank,
              wrestler_id       AS wrestlerId
       FROM season_entries
       WHERE published_date = ?
       ORDER BY rank, raw_source_string`,
    )
    .all(sourceId, weight, season, date) as RankingRow[]
}

// seasonSeries groups a whole weight/season into one rank-over-week line per
// resolved wrestler (the bump chart's data). Unresolved entries (NULL
// wrestler_id) have no identity to string a line through, so they are
// excluded — they still appear in editionEntries. Weeks come from the same
// derivation as listDates; a week a wrestler went unranked is simply absent
// from points (render as a gap, never interpolate). name/school are the
// latest raw spellings, for labeling only.
export function seasonSeries(
  db: Db,
  sourceId: number,
  weight: number,
  season: number,
): WrestlerSeries[] {
  const rows = db
    .prepare(
      `SELECT s.published_date AS date,
              e.rank,
              e.wrestler_id       AS wrestlerId,
              e.raw_source_string AS name,
              e.raw_school        AS school
       FROM ranking_entries e
       JOIN snapshots s ON s.id = e.snapshot_id
       WHERE s.source_id = ? AND s.weight_class = ? AND s.season = ? AND e.wrestler_id IS NOT NULL
       ORDER BY s.published_date, e.rank, e.raw_source_string`,
    )
    .all(sourceId, weight, season) as Array<{
    date: string
    rank: number
    wrestlerId: number
    name: string
    school: string | null
  }>

  const weekByDate = new Map(listDates(db, sourceId, weight, season).map((d) => [d.date, d.week]))

  const byWrestler = new Map<number, WrestlerSeries>()
  for (const row of rows) {
    let series = byWrestler.get(row.wrestlerId)
    if (!series) {
      series = { wrestlerId: row.wrestlerId, name: row.name, school: row.school, points: [] }
      byWrestler.set(row.wrestlerId, series)
    }
    series.name = row.name
    series.school = row.school
    series.points.push({ week: weekByDate.get(row.date)!, rank: row.rank })
  }
  return [...byWrestler.values()]
}
