import { SOURCES, DEFAULT_SOURCE, type RankingSource } from '../../utils/sources'

// resolveSource maps a ?source= slug to its RankingSource, defaulting to
// FloWrestling when omitted. Returns null for an unknown slug — callers
// should 404, not silently fall back, so a bad link never masquerades as the
// default source's data.
export function resolveSource(slug: string | string[] | undefined): RankingSource | null {
  if (slug === undefined) return DEFAULT_SOURCE
  const s = Array.isArray(slug) ? slug[0] : slug
  return SOURCES.find((src) => src.slug === s) ?? null
}
