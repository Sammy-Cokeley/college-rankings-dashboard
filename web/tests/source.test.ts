import { describe, expect, it } from 'vitest'
import { resolveSource } from '../server/utils/source'
import { DEFAULT_SOURCE, SOURCES } from '../utils/sources'

describe('resolveSource', () => {
  it('defaults to FloWrestling when no slug is given', () => {
    expect(resolveSource(undefined)).toEqual(DEFAULT_SOURCE)
  })

  it('resolves a known slug', () => {
    expect(resolveSource('fan-poll')).toEqual(SOURCES.find((s) => s.slug === 'fan-poll'))
  })

  it('returns null for an unknown slug — callers must 404, not silently fall back', () => {
    expect(resolveSource('not-a-real-source')).toBeNull()
  })

  it('takes the first value when given an array (repeated ?source= params)', () => {
    expect(resolveSource(['fan-poll', 'flowrestling'])).toEqual(
      SOURCES.find((s) => s.slug === 'fan-poll'),
    )
  })
})
