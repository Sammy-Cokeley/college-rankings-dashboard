import { describe, expect, it } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import TrajectoryChart from '../components/TrajectoryChart.vue'
import type { WrestlerSeries } from '../types/rankings'
import { editionDates } from './fixtures/rankings'

const contiguous: WrestlerSeries = {
  wrestlerId: 201,
  name: 'Full Season',
  school: 'Iowa',
  points: [
    { week: 1, rank: 1 },
    { week: 2, rank: 2 },
    { week: 3, rank: 1 },
  ],
}

// Ranked weeks 1 and 3, unranked week 2: the line must break, never bridge.
const gapped: WrestlerSeries = {
  wrestlerId: 202,
  name: 'Gap Season',
  school: 'Penn State',
  points: [
    { week: 1, rank: 5 },
    { week: 3, rank: 4 },
  ],
}

function mountChart(selected: number[] = []) {
  return mountSuspended(TrajectoryChart, {
    props: { weeks: editionDates, series: [contiguous, gapped], selected },
  })
}

describe('TrajectoryChart', () => {
  it('renders SVG geometry as real attributes (the .attr bindings)', async () => {
    const wrapper = await mountChart()
    expect(wrapper.find('svg').attributes('viewBox')).toBe('0 0 860 440')
    const line = wrapper.find('polyline.line')
    // three points → "x1,y1 x2,y2 x3,y3"
    expect(line.attributes('points')!.trim().split(' ')).toHaveLength(3)
  })

  it('draws one visible polyline per contiguous segment and dots for lone weeks', async () => {
    const wrapper = await mountChart()
    // contiguous: 1 polyline; gapped: 0 polylines, 2 lone dots
    expect(wrapper.findAll('polyline.line')).toHaveLength(1)
    expect(wrapper.findAll('circle.dot.lone')).toHaveLength(2)
    // hit targets cover every segment of both series: 1 + 2
    expect(wrapper.findAll('polyline.hit')).toHaveLength(3)
  })

  it('emits toggle with the wrestlerId when a line is clicked', async () => {
    const wrapper = await mountChart()
    await wrapper.find('polyline.hit').trigger('click')
    expect(wrapper.emitted('toggle')).toEqual([[201]])
  })

  it('classes the selected series by selection order for the palette', async () => {
    const wrapper = await mountChart([202, 201])
    const groups = wrapper.findAll('g.series')
    const byLabel = (id: number) =>
      groups.find((g) => g.find('polyline.hit title').text().startsWith(id === 201 ? 'Full' : 'Gap'))!
    expect(byLabel(202).classes()).toContain('sel-0')
    expect(byLabel(201).classes()).toContain('sel-1')
  })
})
