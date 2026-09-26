import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { mockNuxtImport, mountSuspended, registerEndpoint } from '@nuxt/test-utils/runtime'
import ProfilePage from '../pages/profile.vue'
import type { BallotSubmission } from '../types/ballots'

// Same vi.hoisted pattern auth-pages.nuxt.test.ts uses for navigateTo — here
// it needs to actually be asserted against (the redirect IS the behavior
// under test for a logged-out visitor), not just silence a real navigation.
const { navigateTo } = vi.hoisted(() => ({ navigateTo: vi.fn() }))

const flags = { loggedIn: false, displayName: undefined as string | undefined, email: 'user@example.com' }

mockNuxtImport('useUserSession', () => () => ({
  loggedIn: ref(flags.loggedIn),
  user: ref(flags.loggedIn ? { displayName: flags.displayName, email: flags.email } : null),
}))
mockNuxtImport('navigateTo', () => navigateTo)

let historyResponse: BallotSubmission[]
registerEndpoint('/api/profile/ballots', () => historyResponse)

beforeEach(() => {
  flags.loggedIn = false
  flags.displayName = undefined
  historyResponse = []
  navigateTo.mockClear()
})

describe('profile.vue', () => {
  it('redirects to login when logged out — the first hard-gated page in the app', async () => {
    await mountSuspended(ProfilePage)
    expect(navigateTo).toHaveBeenCalledWith('/login?redirect=/profile')
  })

  it('shows the empty state with no submissions', async () => {
    flags.loggedIn = true
    const wrapper = await mountSuspended(ProfilePage)
    await vi.waitFor(() => expect(wrapper.text()).toContain('No submissions yet'))
    expect(navigateTo).not.toHaveBeenCalled()
  })

  it('lists submissions newest first, each with its ranked picks', async () => {
    flags.loggedIn = true
    historyResponse = [
      {
        id: 2,
        weightClass: 133,
        submittedAt: '2026-09-20T12:00:00Z',
        entries: [{ rank: 1, wrestlerId: 10, name: 'Later Pick', school: 'Iowa' }],
      },
      {
        id: 1,
        weightClass: 125,
        submittedAt: '2026-09-10T12:00:00Z',
        entries: [{ rank: 1, wrestlerId: 20, name: 'Earlier Pick', school: 'Penn State' }],
      },
    ]
    const wrapper = await mountSuspended(ProfilePage)
    await vi.waitFor(() => expect(wrapper.find('.history-list').exists()).toBe(true))

    const items = wrapper.findAll('.history-list > li')
    expect(items).toHaveLength(2)
    expect(items[0]!.text()).toContain('133 lbs')
    expect(items[0]!.text()).toContain('Later Pick')
    expect(items[1]!.text()).toContain('125 lbs')
    expect(items[1]!.text()).toContain('Earlier Pick')
  })

  it('shows the display name, falling back to email when absent', async () => {
    flags.loggedIn = true
    flags.displayName = 'Coach K'
    const wrapper = await mountSuspended(ProfilePage)
    await vi.waitFor(() => expect(wrapper.text()).toContain('Coach K'))
  })

  it('falls back to email when there is no display name', async () => {
    flags.loggedIn = true
    const wrapper = await mountSuspended(ProfilePage)
    await vi.waitFor(() => expect(wrapper.text()).toContain('user@example.com'))
  })
})
