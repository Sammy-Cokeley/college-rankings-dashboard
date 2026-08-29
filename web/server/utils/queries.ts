import type postgres from 'postgres'
import type { EditionDate, RankingRow, WrestlerSeries } from '../../types/rankings'

export type Db = postgres.Sql

// getSeason returns the newest season for one source, or null when that
// source has no snapshots yet. Scoped to sourceId, not global — sources
// don't share a season number in lockstep (e.g. FloWrestling's newest season
// is last year's completed backfill, 2026, while the Fan Poll's is the
// current roster season, 2027; schema.md §10). An unscoped MAX(season)
// worked by accident in v0 (exactly one source existed), and would silently
// point every source at whichever one happened to have the newest data.
export async function getSeason(db: Db, sourceId: number): Promise<number | null> {
  const rows = await db<{ season: number | null }[]>`
    SELECT MAX(season) AS season FROM snapshots WHERE source_id = ${sourceId}`
  return rows[0]?.season ?? null
}

// getSourceId resolves a source by its seeded canonical name.
export async function getSourceId(db: Db, name: string): Promise<number | null> {
  const rows = await db<{ id: number }[]>`SELECT id FROM sources WHERE name = ${name}`
  return rows[0]?.id ?? null
}

// listDates returns every edition date for one source/weight/season, ascending,
// with the display week derived as the 1-based position (schema.md §4: the
// published_date is the time spine; week numbers are an app-level label).
export async function listDates(
  db: Db,
  sourceId: number,
  weight: number,
  season: number,
): Promise<EditionDate[]> {
  const rows = await db<{ date: string }[]>`
    SELECT DISTINCT published_date AS date
    FROM snapshots
    WHERE source_id = ${sourceId} AND weight_class = ${weight} AND season = ${season}
    ORDER BY published_date`
  return rows.map((r, i) => ({ date: r.date, week: i + 1 }))
}

// latestDate returns the newest edition date for one source/weight/season.
export async function latestDate(
  db: Db,
  sourceId: number,
  weight: number,
  season: number,
): Promise<string | null> {
  const rows = await db<{ date: string | null }[]>`
    SELECT MAX(published_date) AS date
    FROM snapshots
    WHERE source_id = ${sourceId} AND weight_class = ${weight} AND season = ${season}`
  return rows[0]?.date ?? null
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
// wrestler_id) get a NULL prev_rank via CASE. Postgres's LAG lumps all NULLs
// into one partition, which would fabricate movement between unrelated
// unresolved wrestlers.
export async function editionEntries(
  db: Db,
  sourceId: number,
  weight: number,
  season: number,
  date: string,
): Promise<RankingRow[]> {
  const rows = await db<RankingRow[]>`
    WITH season_entries AS (
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
      WHERE s.source_id = ${sourceId} AND s.weight_class = ${weight} AND s.season = ${season}
    )
    SELECT rank,
           raw_source_string AS name,
           raw_school        AS school,
           raw_grade         AS grade,
           prev_rank         AS "prevRank",
           wrestler_id       AS "wrestlerId"
    FROM season_entries
    WHERE published_date = ${date}
    ORDER BY rank, raw_source_string`
  return rows
}

// seasonSeries groups a whole weight/season into one rank-over-week line per
// resolved wrestler (the bump chart's data). Unresolved entries (NULL
// wrestler_id) have no identity to string a line through, so they are
// excluded — they still appear in editionEntries. Weeks come from the same
// derivation as listDates; a week a wrestler went unranked is simply absent
// from points (render as a gap, never interpolate). name/school are the
// latest raw spellings, for labeling only.
export async function seasonSeries(
  db: Db,
  sourceId: number,
  weight: number,
  season: number,
): Promise<WrestlerSeries[]> {
  const rows = await db<
    Array<{
      date: string
      rank: number
      wrestlerId: number
      name: string
      school: string | null
    }>
  >`
    SELECT s.published_date AS date,
           e.rank,
           e.wrestler_id       AS "wrestlerId",
           e.raw_source_string AS name,
           e.raw_school        AS school
    FROM ranking_entries e
    JOIN snapshots s ON s.id = e.snapshot_id
    WHERE s.source_id = ${sourceId} AND s.weight_class = ${weight} AND s.season = ${season}
      AND e.wrestler_id IS NOT NULL
    ORDER BY s.published_date, e.rank, e.raw_source_string`

  const weekByDate = new Map(
    (await listDates(db, sourceId, weight, season)).map((d) => [d.date, d.week]),
  )

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
