import { beforeEach, describe, expect, it } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { mockNuxtImport, mountSuspended, registerEndpoint } from '@nuxt/test-utils/runtime'
import { createError, getQuery, type H3Event } from 'h3'
import IndexPage from '../pages/index.vue'
import { rankingRow, rankingsOverview, seasonSeries } from './fixtures/rankings'
import { WEIGHT_CLASSES } from '../utils/weights'

// Movers reads its source from route.query.source and switches via
// navigateTo — same reactive-stub need as weight-page.nuxt.test.ts's
// router.replace stub, adapted for navigateTo (auth-pages.nuxt.test.ts
// mocks navigateTo too, but only as an assertion spy; here it needs to
// actually update the stub so the movers fetch reacts to it).
const routeStub = reactive({ query: {} as Record<string, string> })
mockNuxtImport('useRoute', () => () => routeStub)
mockNuxtImport('navigateTo', () => (to: { query?: Record<string, string> }) => {
  routeStub.query = to.query ?? {}
})

beforeEach(() => {
  routeStub.query = {}
})

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

  it('switches the movers source on tab click and marks it aria-current', async () => {
    // One handler, branching on the query param — the same shape the real
    // /api/rankings endpoint uses (server/api/rankings/index.get.ts).
    overviewHandler = (event) => {
      const source = getQuery(event).source
      if (source === 'intermat') {
        return rankingsOverview([weightEdition(125, [rankingRow(1, { name: 'Rock', prevRank: 5 })])]) // up 4
      }
      return rankingsOverview([weightEdition(125, mixedWeight)]) // flowrestling default: up 3 / up 7
    }
    seriesHandler = () => seasonSeries(125, mixedWeight)

    const wrapper = await mountSuspended(IndexPage)
    await settle(wrapper)

    const tabs = () => wrapper.findAll('.movers nav.source-tabs button')
    expect(tabs().find((b) => b.text() === 'FloWrestling')!.attributes('aria-current')).toBe('true')
    expect(wrapper.findAll('.movers-row')).toHaveLength(2) // mixedWeight's two risers

    await tabs().find((b) => b.text() === 'InterMat')!.trigger('click')
    await settle(wrapper)

    expect(tabs().find((b) => b.text() === 'InterMat')!.attributes('aria-current')).toBe('true')
    expect(tabs().find((b) => b.text() === 'FloWrestling')!.attributes('aria-current')).toBeUndefined()
    const rows = wrapper.findAll('.movers-row')
    expect(rows).toHaveLength(1)
    expect(rows[0]!.text()).toContain('Rock')
    expect(rows[0]!.text()).toContain('▲4')
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
