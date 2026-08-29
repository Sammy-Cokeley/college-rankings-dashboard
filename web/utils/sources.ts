// The site's ranking sources — shared between server routes (query/validate
// against) and pages (build the source-selector UI). v0 was single-source by
// design (docs/decisions.md); the Fan Poll (Phase 4 of the community-ballots
// feature) is the first second source, published as an ordinary `sources`
// row + snapshots/ranking_entries — same shape as FloWrestling — so it rides
// the existing display stack instead of needing its own.
//
// `slug` is the URL-safe ?source= query value; `name` is the exact string
// seeded into `sources.name` (db/seed/*.sql) that every query joins against.
export interface RankingSource {
  name: string
  slug: string
}

export const SOURCES: readonly RankingSource[] = [
  { name: 'FloWrestling', slug: 'flowrestling' },
  { name: 'Fan Poll', slug: 'fan-poll' },
]

export const DEFAULT_SOURCE = SOURCES[0]!
