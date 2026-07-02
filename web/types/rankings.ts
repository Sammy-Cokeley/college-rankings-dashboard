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
