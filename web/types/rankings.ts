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
}

export interface WeightRankings {
  source: string
  edition: Edition
  dates: EditionDate[] // all editions for the week selector
}

export interface RankingsOverview {
  source: string
  season: number
  weights: Edition[] // latest edition per weight class
}
