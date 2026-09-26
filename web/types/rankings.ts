// Shared response shapes for the Nitro API routes and the pages that consume
// them. Names/schools/grades are the point-in-time raw values as published by
// the source that week (schema.md §1) — never the canonical wrestler's current
// school.

export interface EditionDate {
  date: string // ISO-8601 published_date
  week: number // 1-based index within the season's ascending dates (schema.md §4)
}

export interface RankingRow {
  rank: number // published verbatim; NOT unique within an edition (schema.md §7)
  name: string // raw_source_string
  school: string | null // raw_school
  grade: string | null // raw_grade
  prevRank: number | null // LAG over published_date (schema.md §5); null = first appearance
  wrestlerId: number | null // canonical identity; null until resolved
  // Cross-weight annotation (decisions.md "Cross-weight annotation"): when
  // this is a first appearance at THIS weight (prevRank null), the
  // wrestler's most recent appearance at a DIFFERENT weight this source+
  // season, if any. Always null when prevRank is non-null — irrelevant once
  // movement is already shown. Unresolved entries never get one (no
  // identity to follow across weights).
  prevWeight: { weight: number; rank: number } | null
  conference: string | null // raw_conference; null for sources that don't publish it
  record: string | null // raw_record; null for sources that don't publish it
}

export interface SeriesPoint {
  week: number
  rank: number
}

// One wrestler's rank-over-week line for a weight/season. Weeks a wrestler
// went unranked are simply absent — consumers render gaps, never interpolate.
// name/school are the LATEST raw spellings (for labeling); each underlying
// entry still keeps its own verbatim values.
export interface WrestlerSeries {
  wrestlerId: number
  name: string
  school: string | null
  points: SeriesPoint[]
}

export interface SeasonSeries {
  source: string
  weight: number
  season: number
  weeks: EditionDate[]
  series: WrestlerSeries[]
}

export interface Edition {
  weight: number
  season: number
  date: string
  week: number
  entries: RankingRow[]
  // The exact page this edition was scraped from (snapshots.source_url),
  // when the pipeline recorded one — null for Fan Poll (not an external
  // source) and for data ingested before this column existed. Distinct from
  // WeightRankings/RankingsOverview's sourceUrl, which is each source's
  // stable homepage: this is the specific edition's page, when known.
  url: string | null
}

export interface WeightRankings {
  source: string
  sourceUrl: string | null // attribution link (utils/sources.ts); null for Fan Poll
  edition: Edition
  dates: EditionDate[] // all editions for the week selector
}

export interface RankingsOverview {
  source: string
  sourceUrl: string | null // attribution link (utils/sources.ts); null for Fan Poll
  season: number
  weights: Edition[] // latest edition per weight class
}
