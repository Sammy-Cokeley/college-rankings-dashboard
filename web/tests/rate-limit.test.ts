import { describe, expect, it } from 'vitest'
import { checkRateLimit } from '../server/utils/rate-limit'

describe('checkRateLimit', () => {
  it('allows up to max hits within the window', () => {
    const key = `test-allow-${Math.random()}`
    for (let i = 0; i < 5; i++) {
      expect(checkRateLimit(key, 5, 60_000)).toBe(true)
    }
  })

  it('rejects the hit that exceeds max within the window', () => {
    const key = `test-reject-${Math.random()}`
    for (let i = 0; i < 5; i++) {
      checkRateLimit(key, 5, 60_000)
    }
    expect(checkRateLimit(key, 5, 60_000)).toBe(false)
  })

  it('scopes limits per key — a different key starts fresh', () => {
    const keyA = `test-scope-a-${Math.random()}`
    const keyB = `test-scope-b-${Math.random()}`
    for (let i = 0; i < 5; i++) checkRateLimit(keyA, 5, 60_000)
    expect(checkRateLimit(keyA, 5, 60_000)).toBe(false)
    expect(checkRateLimit(keyB, 5, 60_000)).toBe(true)
  })

  it('an old hit outside the window does not count against the limit', () => {
    const key = `test-window-${Math.random()}`
    // A window of 1ms means the first hit is already "old" by the time the
    // second one is checked — proves pruning, not just a static counter.
    expect(checkRateLimit(key, 1, 1)).toBe(true)
    const start = Date.now()
    while (Date.now() - start < 5) {
      /* busy-wait past the 1ms window */
    }
    expect(checkRateLimit(key, 1, 1)).toBe(true)
  })
})
