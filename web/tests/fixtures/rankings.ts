import type {
  Edition,
  EditionDate,
  RankingRow,
  RankingsOverview,
  SeasonSeries,
  WeightRankings,
  WrestlerSeries,
} from '../../types/rankings'

// Deterministic id scheme: wrestlerId = 100 + rank. Keeps test assertions
// readable ("rank 12" ↔ id 112) without a lookup table.
export function rankingRow(rank: number, overrides: Partial<RankingRow> = {}): RankingRow {
  return {
    rank,
    name: `Wrestler ${rank}`,
    school: `School ${rank}`,
    grade: 'SR',
    prevRank: rank,
    wrestlerId: 100 + rank,
    ...overrides,
  }
}

export const editionDates: EditionDate[] = [
  { date: '2026-01-01', week: 1 },
  { date: '2026-01-08', week: 2 },
  { date: '2026-01-15', week: 3 },
]

export function weightRankings(weight: number, entries: RankingRow[]): WeightRankings {
  return {
    source: 'FloWrestling',
    edition: {
      weight,
      season: 2026,
      date: editionDates[editionDates.length - 1]!.date,
      week: editionDates.length,
      entries,
    },
    dates: editionDates,
  }
}

export function rankingsOverview(weights: Edition[]): RankingsOverview {
  return { source: 'FloWrestling', season: 2026, weights }
}

// One single-point series per resolved entry, so every fixture wrestlerId is
// "known" to the ?sel= validation in [weight].vue.
export function seasonSeries(weight: number, entries: RankingRow[]): SeasonSeries {
  const series: WrestlerSeries[] = entries
    .filter((r) => r.wrestlerId !== null)
    .map((r) => ({
      wrestlerId: r.wrestlerId!,
      name: r.name,
      school: r.school,
      points: [{ week: editionDates.length, rank: r.rank }],
    }))
  return { source: 'FloWrestling', weight, season: 2026, weeks: editionDates, series }
}
