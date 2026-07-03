import type { SeriesPoint } from '~/types/rankings'

export interface ChartBox {
  width: number
  height: number
  padTop: number
  padRight: number
  padBottom: number
  padLeft: number
}

export interface ChartScales {
  x: (week: number) => number
  y: (rank: number) => number
}

// chartScales maps week 1..weekCount onto the padded x extent and rank
// 1..maxRank onto the padded y extent with rank 1 at the TOP (a rankings
// chart reads "up is better"). Degenerate domains pin to the start edge.
export function chartScales(weekCount: number, maxRank: number, box: ChartBox): ChartScales {
  const innerW = box.width - box.padLeft - box.padRight
  const innerH = box.height - box.padTop - box.padBottom
  return {
    x: (week) => box.padLeft + (weekCount > 1 ? ((week - 1) / (weekCount - 1)) * innerW : 0),
    y: (rank) => box.padTop + (maxRank > 1 ? ((rank - 1) / (maxRank - 1)) * innerH : 0),
  }
}

// seriesSegments splits a wrestler's points into runs of consecutive weeks.
// A missing week (unranked, or ranked at another weight that week) breaks the
// line — gaps are honest, never interpolated. Points must be week-ascending,
// which is how seasonSeries produces them.
export function seriesSegments(points: SeriesPoint[]): SeriesPoint[][] {
  const segments: SeriesPoint[][] = []
  for (const point of points) {
    const current = segments[segments.length - 1]
    const prev = current?.[current.length - 1]
    if (prev && point.week === prev.week + 1) {
      current.push(point)
    } else {
      segments.push([point])
    }
  }
  return segments
}

// polylinePoints renders one segment as an SVG points string.
export function polylinePoints(segment: SeriesPoint[], scales: ChartScales): string {
  return segment
    .map((p) => `${round2(scales.x(p.week))},${round2(scales.y(p.rank))}`)
    .join(' ')
}

function round2(n: number): number {
  return Math.round(n * 100) / 100
}
