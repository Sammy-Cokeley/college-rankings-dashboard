import { describe, expect, it } from 'vitest'
import { movement } from '../utils/movement'
import { isWeightClass } from '../utils/weights'

describe('movement', () => {
  it('is "new" on first appearance (null prevRank)', () => {
    expect(movement(4, null)).toEqual({ kind: 'new', delta: 0 })
  })

  it('is "up" with a positive delta when the rank number drops', () => {
    expect(movement(1, 3)).toEqual({ kind: 'up', delta: 2 })
  })

  it('is "down" with a positive delta when the rank number climbs', () => {
    expect(movement(5, 2)).toEqual({ kind: 'down', delta: 3 })
  })

  it('is "even" when the rank holds', () => {
    expect(movement(7, 7)).toEqual({ kind: 'even', delta: 0 })
  })
})

describe('isWeightClass', () => {
  it('accepts the ten NCAA DI weights and nothing else', () => {
    expect(isWeightClass(125)).toBe(true)
    expect(isWeightClass(285)).toBe(true)
    expect(isWeightClass(158)).toBe(false)
    expect(isWeightClass(0)).toBe(false)
  })
})
