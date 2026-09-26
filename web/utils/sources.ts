// The site's ranking sources — shared between server routes (query/validate
// against) and pages (build the source-selector UI). v0 was single-source by
// design (docs/decisions.md); the Fan Poll (Phase 4 of the community-ballots
// feature) is the first second source, published as an ordinary `sources`
// row + snapshots/ranking_entries — same shape as FloWrestling — so it rides
// the existing display stack instead of needing its own.
//
// `slug` is the URL-safe ?source= query value; `name` is the exact string
// seeded into `sources.name` (db/seed/*.sql) that every query joins against.
// `url` is the attribution link shown next to the source name on the
// rankings pages (decisions.md "posture mitigation: attribute prominently
// and link every ranking back") — the site's stable homepage, not a
// season-specific deep link that would go stale; null for Fan Poll, which
// isn't an external source. The `sources.url` DB column also exists but
// nothing reads it — this static list is the actual source of truth for
// every other piece of source metadata already, so `url` belongs here too
// rather than adding a DB round trip for one more static field.
export interface RankingSource {
  name: string
  slug: string
  url: string | null
}

export const SOURCES: readonly RankingSource[] = [
  { name: 'FloWrestling', slug: 'flowrestling', url: 'https://www.flowrestling.org' },
  { name: 'Fan Poll', slug: 'fan-poll', url: null },
  { name: 'InterMat', slug: 'intermat', url: 'https://intermatwrestle.com' },
]

export const DEFAULT_SOURCE = SOURCES[0]!
