import { describe, expect, it } from 'vitest'
import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { mountSuspended, registerEndpoint } from '@nuxt/test-utils/runtime'
import { createError, type H3Event } from 'h3'
import IndexPage from '../pages/index.vue'
import { rankingRow, rankingsOverview, seasonSeries } from './fixtures/rankings'
import { WEIGHT_CLASSES } from '../utils/weights'

// registerEndpoint wires a path to one handler for the whole file — each
// test case reassigns these instead of re-registering (weight-page.nuxt.test.ts,
// auth-pages.nuxt.test.ts use the same pattern).
let overviewHandler: (event: H3Event) => unknown
registerEndpoint('/api/rankings', (event) => overviewHandler(event))

let seriesHandler: (event: H3Event) => unknown
registerEndpoint('/api/rankings/125/series', (event) => seriesHandler(event))

async function settle(wrapper: VueWrapper<unknown>) {
  await flushPromises()
  await wrapper.vm.$nextTick()
}

// A mix of up/down/new/even so the movers teaser's filter+sort is actually
// exercised, not just "renders whatever comes back."
const mixedWeight = [
  rankingRow(1, { prevRank: 4 }), // up 3
  rankingRow(2, { prevRank: 1 }), // down 1
  rankingRow(3, { prevRank: null }), // new
  rankingRow(4, { prevRank: 4 }), // even
  rankingRow(5, { prevRank: 12 }), // up 7 — biggest riser
]

describe('index.vue (landing page)', () => {
  it('links every weight class straight to its ballot builder', async () => {
    overviewHandler = () => rankingsOverview([])
    seriesHandler = () => {
      throw createError({ statusCode: 503, statusMessage: 'No ranking data ingested yet' })
    }

    const wrapper = await mountSuspended(IndexPage)
    await settle(wrapper)

    for (const w of WEIGHT_CLASSES) {
      const link = wrapper.find(`a[href="/ballot/${w}"]`)
      expect(link.exists()).toBe(true)
    }
  })

  it('shows only risers, sorted biggest delta first, capped at 5', async () => {
    // Three weights, each contributing the same two risers (up 3, up 7) plus
    // a down/new/even row that must never appear — 6 up-rows total, so the
    // top-5 cap actually gets exercised.
    overviewHandler = () =>
      rankingsOverview([
        weightEdition(125, mixedWeight),
        weightEdition(133, mixedWeight),
        weightEdition(141, mixedWeight),
      ])
    seriesHandler = () => seasonSeries(125, mixedWeight)

    const wrapper = await mountSuspended(IndexPage)
    await settle(wrapper)

    const rows = wrapper.findAll('.movers-row')
    expect(rows).toHaveLength(5) // capped from 6
    expect(rows[0]!.text()).toContain('▲7')
    expect(rows[1]!.text()).toContain('▲7')
    expect(rows[2]!.text()).toContain('▲7')
    expect(rows[3]!.text()).toContain('▲3')
    expect(rows[4]!.text()).toContain('▲3')
  })

  it('degrades quietly (no crash) when the movers/chart endpoints fail', async () => {
    overviewHandler = () => {
      throw createError({ statusCode: 503, statusMessage: 'No ranking data ingested yet' })
    }
    seriesHandler = () => {
      throw createError({ statusCode: 404, statusMessage: 'No editions for this weight class' })
    }

    const wrapper = await mountSuspended(IndexPage)
    await settle(wrapper)

    expect(wrapper.find('.hero h1').exists()).toBe(true)
    expect(wrapper.find(`a[href="/ballot/${WEIGHT_CLASSES[0]}"]`).exists()).toBe(true)
    expect(wrapper.find('.movers-row').exists()).toBe(false)
    expect(wrapper.find('.hero-chart').exists()).toBe(false)
  })
})

function weightEdition(weight: number, entries: ReturnType<typeof rankingRow>[]) {
  return { weight, season: 2026, date: '2026-01-15', week: 3, entries }
}
