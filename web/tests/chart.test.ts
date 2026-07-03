import { describe, expect, it } from 'vitest'
import { chartScales, polylinePoints, seriesSegments, type ChartBox } from '../utils/chart'

const box: ChartBox = { width: 100, height: 60, padTop: 0, padRight: 0, padBottom: 0, padLeft: 0 }

describe('chartScales', () => {
  it('maps the week range onto the padded x extent', () => {
    const { x } = chartScales(5, 10, box)
    expect(x(1)).toBe(0)
    expect(x(5)).toBe(100)
    expect(x(3)).toBe(50)
  })

  it('maps rank 1 to the top and maxRank to the bottom (inverted y)', () => {
    const { y } = chartScales(5, 10, box)
    expect(y(1)).toBe(0)
    expect(y(10)).toBe(60)
    expect(y(1)).toBeLessThan(y(2))
  })

  it('respects padding', () => {
    const padded: ChartBox = { width: 100, height: 60, padTop: 5, padRight: 10, padBottom: 15, padLeft: 20 }
    const { x, y } = chartScales(2, 2, padded)
    expect(x(1)).toBe(20)
    expect(x(2)).toBe(90)
    expect(y(1)).toBe(5)
    expect(y(2)).toBe(45)
  })

  it('degenerate domains (one week, one rank) stay finite', () => {
    const { x, y } = chartScales(1, 1, box)
    expect(Number.isFinite(x(1))).toBe(true)
    expect(Number.isFinite(y(1))).toBe(true)
  })
})

describe('seriesSegments', () => {
  it('keeps consecutive weeks as one segment', () => {
    const pts = [
      { week: 1, rank: 3 },
      { week: 2, rank: 2 },
      { week: 3, rank: 2 },
    ]
    expect(seriesSegments(pts)).toEqual([pts])
  })

  it('splits on a gap week and never interpolates across it', () => {
    const segments = seriesSegments([
      { week: 1, rank: 3 },
      { week: 2, rank: 3 },
      { week: 4, rank: 5 },
    ])
    expect(segments).toEqual([
      [
        { week: 1, rank: 3 },
        { week: 2, rank: 3 },
      ],
      [{ week: 4, rank: 5 }],
    ])
  })

  it('handles isolated single-week appearances', () => {
    expect(
      seriesSegments([
        { week: 1, rank: 9 },
        { week: 3, rank: 8 },
        { week: 5, rank: 9 },
      ]),
    ).toHaveLength(3)
  })

  it('returns nothing for an empty series', () => {
    expect(seriesSegments([])).toEqual([])
  })
})

describe('polylinePoints', () => {
  it('renders scaled x,y pairs', () => {
    const scales = chartScales(5, 10, box)
    const points = polylinePoints(
      [
        { week: 1, rank: 1 },
        { week: 5, rank: 10 },
      ],
      scales,
    )
    expect(points).toBe('0,0 100,60')
  })
})
