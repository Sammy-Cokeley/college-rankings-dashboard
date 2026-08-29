import { beforeEach, describe, expect, it } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { mockNuxtImport, mountSuspended, registerEndpoint } from '@nuxt/test-utils/runtime'
import WeightPage from '../pages/[weight].vue'
import { rankingRow, seasonSeries, weightRankings } from './fixtures/rankings'

// Under mountSuspended the component's useRoute() is a static snapshot: an
// in-component router.replace never reaches it, so ?sel= interactions can't
// be tested through the real router. Selection state LIVES in the route, so
// we stub the pair with a reactive route: the component reads route.query and
// writes via router.replace — the stub closes that loop and nothing else.
const routeStub = reactive({
  params: { weight: '125' } as Record<string, string>,
  query: {} as Record<string, string>,
  path: '/125',
  fullPath: '/125',
})

mockNuxtImport('useRoute', () => () => routeStub)
// Nuxt's own plugins and the test-utils entry also grab this router, so the
// stub needs the guard-hook surface as no-ops, not just replace().
mockNuxtImport('useRouter', () => () => ({
  replace: (to: { query?: Record<string, string> }) => {
    routeStub.query = to.query ?? {}
  },
  push: (to: { query?: Record<string, string> }) => {
    routeStub.query = to.query ?? {}
  },
  beforeEach: () => () => {},
  beforeResolve: () => () => {},
  afterEach: () => () => {},
  onError: () => () => {},
  resolve: (to: unknown) => to,
  currentRoute: { value: routeStub },
}))

function setRoute(weight: number, query: Record<string, string> = {}) {
  routeStub.params = { weight: String(weight) }
  routeStub.query = query
  routeStub.path = `/${weight}`
  routeStub.fullPath = `/${weight}`
}

beforeEach(() => setRoute(125))

// 14 ranked: 1-10 above the fold, 11-14 behind it. Iowa appears at rank 2
// (above fold) and rank 12 (below fold) to exercise school-group selection
// and the auto-expand watcher in one gesture. Rank 5 is unresolved.
const roster = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14].map((rank) => {
  if (rank === 2 || rank === 12) return rankingRow(rank, { school: 'Iowa' })
  if (rank === 5) return rankingRow(rank, { wrestlerId: null })
  return rankingRow(rank)
})

registerEndpoint('/api/rankings/125', () => weightRankings(125, roster))
registerEndpoint('/api/rankings/125/series', () => seasonSeries(125, roster))

// Two wrestlers tied at rank 10, one at rank 11: the fold cuts by rank, so a
// tie at the fold rank must never be half-hidden.
const tied = [
  ...[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((rank) => rankingRow(rank)),
  rankingRow(10, { name: 'Tie Ten', wrestlerId: 210, school: 'Tie School' }),
  rankingRow(11),
]

registerEndpoint('/api/rankings/133', () => weightRankings(133, tied))
registerEndpoint('/api/rankings/133/series', () => seasonSeries(133, tied))

async function settle(wrapper: VueWrapper<unknown>) {
  await flushPromises()
  await wrapper.vm.$nextTick()
}

const rows = (wrapper: VueWrapper<unknown>) => wrapper.findAll('tbody tr')
const foldedRows = (wrapper: VueWrapper<unknown>) => wrapper.findAll('tbody tr.folded')
const selectedRows = (wrapper: VueWrapper<unknown>) => wrapper.findAll('tbody tr.selected')
const foldButton = (wrapper: VueWrapper<unknown>) => wrapper.find('.fold-toggle button')

describe('[weight].vue fold', () => {
  it('hides ranks past 10 behind the fold and toggles on the button', async () => {
    const wrapper = await mountSuspended(WeightPage)
    expect(rows(wrapper)).toHaveLength(14)
    expect(foldedRows(wrapper)).toHaveLength(4)
    expect(foldButton(wrapper).text()).toBe('See all 14 ranked ▾')

    await foldButton(wrapper).trigger('click')
    expect(foldedRows(wrapper)).toHaveLength(0)
    expect(foldButton(wrapper).text()).toBe('Show top 10 ▴')
  })

  it('keeps every wrestler tied at the fold rank visible', async () => {
    setRoute(133)
    const wrapper = await mountSuspended(WeightPage)
    expect(rows(wrapper)).toHaveLength(12)
    // only rank 11 folds; both rank-10 rows stay visible
    expect(foldedRows(wrapper)).toHaveLength(1)
    expect(foldedRows(wrapper)[0]!.text()).toContain('Wrestler 11')
  })

  it('mounts already expanded when ?sel= deep-links a below-fold wrestler', async () => {
    setRoute(125, { sel: '112' })
    const wrapper = await mountSuspended(WeightPage)
    expect(foldedRows(wrapper)).toHaveLength(0)
    expect(selectedRows(wrapper)).toHaveLength(1)
  })
})

describe('[weight].vue selection', () => {
  it('toggles a wrestler into and out of ?sel= by row click', async () => {
    const wrapper = await mountSuspended(WeightPage)
    const row1 = rows(wrapper)[0]!

    await row1.trigger('click')
    await settle(wrapper)
    expect(routeStub.query.sel).toBe('101')
    expect(row1.classes()).toContain('selected')

    await row1.trigger('click')
    await settle(wrapper)
    expect(routeStub.query.sel).toBeUndefined()
    expect(selectedRows(wrapper)).toHaveLength(0)
  })

  it('ignores clicks on unresolved rows (no wrestlerId, no line to pin)', async () => {
    const wrapper = await mountSuspended(WeightPage)
    await rows(wrapper)[4]!.trigger('click') // rank 5, wrestlerId null
    await settle(wrapper)
    expect(routeStub.query.sel).toBeUndefined()
    expect(selectedRows(wrapper)).toHaveLength(0)
  })

  it('selects the whole school on school-cell click and auto-expands past the fold', async () => {
    const wrapper = await mountSuspended(WeightPage)
    const iowaCell = rows(wrapper)[1]!.find('.school-toggle')

    await iowaCell.trigger('click')
    await settle(wrapper)
    expect(routeStub.query.sel).toBe('102,112')
    expect(selectedRows(wrapper)).toHaveLength(2)
    // rank 12 is below the fold: selecting it must reveal it
    expect(foldedRows(wrapper)).toHaveLength(0)

    await iowaCell.trigger('click')
    await settle(wrapper)
    expect(routeStub.query.sel).toBeUndefined()
    expect(selectedRows(wrapper)).toHaveLength(0)
  })

  it('drops unknown ids from ?sel= instead of selecting or expanding', async () => {
    setRoute(125, { sel: '999,abc' })
    const wrapper = await mountSuspended(WeightPage)
    expect(selectedRows(wrapper)).toHaveLength(0)
    expect(foldedRows(wrapper)).toHaveLength(4)
  })
})
