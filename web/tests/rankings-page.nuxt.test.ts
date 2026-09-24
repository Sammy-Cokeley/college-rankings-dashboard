import { describe, expect, it } from 'vitest'
import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { mountSuspended, registerEndpoint } from '@nuxt/test-utils/runtime'
import type { H3Event } from 'h3'
import RankingsPage from '../pages/rankings.vue'
import { rankingRow, rankingsOverview } from './fixtures/rankings'

let overviewHandler: (event: H3Event) => unknown
registerEndpoint('/api/rankings', (event) => overviewHandler(event))

function weightEdition(weight: number, entries: ReturnType<typeof rankingRow>[]) {
  return { weight, season: 2026, date: '2026-01-15', week: 3, entries }
}

async function settle(wrapper: VueWrapper<unknown>) {
  await flushPromises()
  await wrapper.vm.$nextTick()
}

function mount() {
  overviewHandler = () =>
    rankingsOverview([
      weightEdition(133, [rankingRow(2, { name: 'Ann' })]),
      weightEdition(125, [rankingRow(1, { name: 'Zed' })]),
    ])
  return mountSuspended(RankingsPage)
}

const headerButton = (wrapper: VueWrapper<unknown>, label: string) =>
  wrapper.findAll('th button').find((b) => b.text().startsWith(label))!

const headerCell = (wrapper: VueWrapper<unknown>, label: string) =>
  headerButton(wrapper, label).element.closest('th')!

const bodyRows = (wrapper: VueWrapper<unknown>) => wrapper.findAll('tbody tr')

describe('rankings.vue sorting', () => {
  it('renders each sortable header as a real button, plain headers untouched', async () => {
    const wrapper = await mount()
    await settle(wrapper)

    for (const label of ['WT', 'RK', 'Wrestler', 'School', 'Move']) {
      expect(headerButton(wrapper, label).element.tagName).toBe('BUTTON')
    }
    // YR has no sort — must stay a plain header with no nested button.
    const yrHeader = wrapper.findAll('th').find((th) => th.text() === 'YR')!
    expect(yrHeader.find('button').exists()).toBe(false)
  })

  it('defaults to weight ascending, reflected in aria-sort', async () => {
    const wrapper = await mount()
    await settle(wrapper)

    expect(headerCell(wrapper, 'WT').getAttribute('aria-sort')).toBe('ascending')
    expect(headerCell(wrapper, 'RK').getAttribute('aria-sort')).toBe('none')
    expect(bodyRows(wrapper)[0]!.text()).toContain('Zed') // weight 125 first
  })

  it('sorts by clicking a header button, and flips direction on a second click', async () => {
    const wrapper = await mount()
    await settle(wrapper)

    await headerButton(wrapper, 'Wrestler').trigger('click')
    await settle(wrapper)
    expect(headerCell(wrapper, 'Wrestler').getAttribute('aria-sort')).toBe('ascending')
    expect(headerCell(wrapper, 'WT').getAttribute('aria-sort')).toBe('none')
    expect(bodyRows(wrapper)[0]!.text()).toContain('Ann') // 'Ann' < 'Zed'

    await headerButton(wrapper, 'Wrestler').trigger('click')
    await settle(wrapper)
    expect(headerCell(wrapper, 'Wrestler').getAttribute('aria-sort')).toBe('descending')
    expect(bodyRows(wrapper)[0]!.text()).toContain('Zed')
  })
})
