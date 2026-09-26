import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import type { VueWrapper } from '@vue/test-utils'
import { mockNuxtImport, mountSuspended, registerEndpoint } from '@nuxt/test-utils/runtime'
import { readBody, type H3Event } from 'h3'
import type { WrestlerOption } from '../types/ballots'
import BallotPage from '../pages/ballot/[weight].vue'

// mockNuxtImport's factory argument runs eagerly, at hoisted-above-imports
// position (the same rule vi.hoisted works around elsewhere) — but only the
// OUTER function here runs eagerly; it just returns another function without
// calling anything. That INNER function is what actually runs `useUserSession()`
// calls resolve to, lazily, once per component setup — by which point 'vue'
// is fully loaded, so ref() is safe to call there.
const flags = { loggedIn: false }
const refreshSession = vi.fn()

mockNuxtImport('useUserSession', () => () => ({ loggedIn: ref(flags.loggedIn), fetch: refreshSession }))
mockNuxtImport('useRoute', () => () => ({ params: { weight: '149' }, query: {}, fullPath: '/ballot/149' }))

const searchResults: WrestlerOption[] = [
  { wrestlerId: 1, name: 'Real Deal', school: 'Iowa', weightClass: 149 },
  { wrestlerId: 2, name: 'Off Weight Guy', school: 'Penn State', weightClass: 133 },
]
registerEndpoint('/api/wrestlers/search', () => searchResults)

let ballotEntries: Array<{ rank: number; wrestlerId: number; name: string; school: string | null }> = []
let patchBody: unknown
registerEndpoint('/api/ballots/149', {
  method: 'GET',
  handler: () => ({ weightClass: 149, season: 2027, updatedAt: null, entries: ballotEntries }),
})
registerEndpoint('/api/ballots/149', {
  method: 'PATCH',
  handler: async (event: H3Event) => {
    patchBody = await readBody(event)
    return { ok: true }
  },
})

let submitCalled: number
registerEndpoint('/api/ballots/149/submit', {
  method: 'POST',
  handler: () => {
    submitCalled += 1
    return { ok: true }
  },
})

beforeEach(() => {
  localStorage.clear()
  flags.loggedIn = false
  ballotEntries = []
  patchBody = undefined
  submitCalled = 0
  refreshSession.mockClear()
})

// @vue/test-utils' find() takes a plain CSS selector — no :has-text()
// pseudo-class (that's a Playwright extension) — so matching a search
// result button by its label needs a manual scan.
function findButtonByText(wrapper: VueWrapper, text: string) {
  const match = wrapper.findAll('button').find((b) => b.text().includes(text))
  if (!match) throw new Error(`no <button> containing ${JSON.stringify(text)}`)
  return match
}

describe('ballot builder — guest', () => {
  it('shows a sign up / log in prompt instead of a save indicator', async () => {
    const wrapper = await mountSuspended(BallotPage)
    await vi.waitFor(() => expect(wrapper.text()).toContain('Sign up'))
    expect(wrapper.text()).toContain('log in')
    expect(wrapper.text()).toContain('save your ballot')
    // Submitting locks in history under an account — meaningless while
    // anonymous, so the button shouldn't exist at all, not just be disabled.
    expect(wrapper.find('.submit-ballot').exists()).toBe(false)
  })

  it('adding a wrestler persists to localStorage, not the server', async () => {
    const wrapper = await mountSuspended(BallotPage)
    await vi.waitFor(() => expect(wrapper.text()).toContain('Real Deal'))

    await findButtonByText(wrapper, 'Real Deal').trigger('click')
    await vi.waitFor(() => {
      const raw = localStorage.getItem('ballot-draft-149')
      expect(raw).toBeTruthy()
    })

    const stored = JSON.parse(localStorage.getItem('ballot-draft-149')!)
    expect(stored).toEqual([{ rank: 1, wrestlerId: 1, name: 'Real Deal', school: 'Iowa' }])
    expect(patchBody).toBeUndefined() // never reached the auth-guarded write API
  })

  it('restores a draft already in localStorage on load', async () => {
    localStorage.setItem(
      'ballot-draft-149',
      JSON.stringify([{ rank: 1, wrestlerId: 1, name: 'Real Deal', school: 'Iowa' }]),
    )
    const wrapper = await mountSuspended(BallotPage)
    await vi.waitFor(() => expect(wrapper.find('.ballot-list').exists()).toBe(true))
    expect(wrapper.find('.ballot-list').text()).toContain('Real Deal')
  })
})

describe('ballot builder — logged in', () => {
  it('loads the existing ballot from the server, not localStorage', async () => {
    ballotEntries = [{ rank: 1, wrestlerId: 2, name: 'Off Weight Guy', school: 'Penn State' }]
    flags.loggedIn = true

    const wrapper = await mountSuspended(BallotPage)
    await vi.waitFor(() => expect(wrapper.find('.ballot-list').exists()).toBe(true))
    expect(wrapper.find('.ballot-list').text()).toContain('Off Weight Guy')
    expect(wrapper.text()).not.toContain('Sign up')
  })

  it('adding a wrestler autosaves to the server, not localStorage', async () => {
    flags.loggedIn = true
    const wrapper = await mountSuspended(BallotPage)
    await vi.waitFor(() => expect(wrapper.text()).toContain('Real Deal'))

    await findButtonByText(wrapper, 'Real Deal').trigger('click')
    await vi.waitFor(() => expect(patchBody).toBeDefined())

    expect(patchBody).toEqual({ wrestlerIds: [1] })
    expect(localStorage.getItem('ballot-draft-149')).toBeNull()
  })

  it('recovers a draft left over from before signing in, and persists it to the server', async () => {
    // The bug this pins: loadEntries used to just overwrite entries with the
    // empty server ballot here, silently discarding whatever a user built
    // anonymously the moment they signed up.
    flags.loggedIn = true
    ballotEntries = [] // fresh server ballot — nothing saved under this account yet
    localStorage.setItem(
      'ballot-draft-149',
      JSON.stringify([{ rank: 1, wrestlerId: 1, name: 'Real Deal', school: 'Iowa' }]),
    )

    const wrapper = await mountSuspended(BallotPage)
    await vi.waitFor(() => expect(wrapper.find('.ballot-list').exists()).toBe(true))
    expect(wrapper.find('.ballot-list').text()).toContain('Real Deal')

    await vi.waitFor(() => expect(patchBody).toEqual({ wrestlerIds: [1] }))
    expect(localStorage.getItem('ballot-draft-149')).toBeNull()
  })

  it('prefers an existing server ballot over a stale draft, and clears the draft either way', async () => {
    flags.loggedIn = true
    ballotEntries = [{ rank: 1, wrestlerId: 2, name: 'Off Weight Guy', school: 'Penn State' }]
    localStorage.setItem(
      'ballot-draft-149',
      JSON.stringify([{ rank: 1, wrestlerId: 1, name: 'Real Deal', school: 'Iowa' }]),
    )

    const wrapper = await mountSuspended(BallotPage)
    await vi.waitFor(() => expect(wrapper.find('.ballot-list').exists()).toBe(true))
    expect(wrapper.find('.ballot-list').text()).toContain('Off Weight Guy')
    expect(wrapper.find('.ballot-list').text()).not.toContain('Real Deal')
    expect(patchBody).toBeUndefined() // never re-saved — the server ballot was already authoritative
    expect(localStorage.getItem('ballot-draft-149')).toBeNull()
  })

  it('disables Submit for an empty ballot', async () => {
    flags.loggedIn = true
    const wrapper = await mountSuspended(BallotPage)
    // Wait for the actual load to finish (empty entries, loaded=true), not
    // just for the button to exist — it renders before loadEntries resolves,
    // while entries.value is still its initial [], which would make this
    // assertion vacuously true even if the disabled binding were broken.
    await vi.waitFor(() => expect(wrapper.text()).toContain('Search above to add wrestlers'))
    expect(wrapper.find('.submit-ballot').attributes('disabled')).toBeDefined()
  })

  it('clicking Submit calls the submit endpoint and shows a submitted indicator', async () => {
    ballotEntries = [{ rank: 1, wrestlerId: 2, name: 'Off Weight Guy', school: 'Penn State' }]
    flags.loggedIn = true

    const wrapper = await mountSuspended(BallotPage)
    // Must wait for the real ballot data to load, not just the button's
    // existence — a disabled button never dispatches a click at all (real
    // browser behavior, matched by jsdom/happy-dom), so clicking it while
    // entries.value is still its pre-load [] would silently no-op.
    await vi.waitFor(() => expect(wrapper.find('.ballot-list').exists()).toBe(true))
    expect(wrapper.find('.submit-ballot').attributes('disabled')).toBeUndefined()

    await wrapper.find('.submit-ballot').trigger('click')
    await vi.waitFor(() => expect(submitCalled).toBe(1))
    // Don't assert on wrapper.text() right off the back of the submitCalled
    // wait: that only proves the mock's handler ran, not that the client's
    // $fetch has resolved and Vue has re-rendered — needs its own poll.
    await vi.waitFor(() => expect(wrapper.text()).toContain('Submitted'))
  })

  it('editing after a submit clears the submitted indicator', async () => {
    ballotEntries = [{ rank: 1, wrestlerId: 2, name: 'Off Weight Guy', school: 'Penn State' }]
    flags.loggedIn = true

    const wrapper = await mountSuspended(BallotPage)
    await vi.waitFor(() => expect(wrapper.find('.ballot-list').exists()).toBe(true))
    await wrapper.find('.submit-ballot').trigger('click')
    await vi.waitFor(() => expect(wrapper.text()).toContain('Submitted'))

    await vi.waitFor(() => expect(wrapper.text()).toContain('Real Deal')) // search results loaded
    await findButtonByText(wrapper, 'Real Deal').trigger('click')
    await vi.waitFor(() => expect(wrapper.text()).not.toContain('Submitted'))
    // Let the edit's debounced autosave actually fire before the test ends —
    // otherwise its 500ms timer outlives this test and pollutes the next
    // one's patchBody once it eventually fires.
    await vi.waitFor(() => expect(patchBody).toBeDefined())
  })

  it('does not autosave on initial load — only after a real edit', async () => {
    ballotEntries = [{ rank: 1, wrestlerId: 2, name: 'Off Weight Guy', school: 'Penn State' }]
    flags.loggedIn = true

    const wrapper = await mountSuspended(BallotPage)
    await vi.waitFor(() => expect(wrapper.find('.ballot-list').exists()).toBe(true))
    // Give the (deliberately absent) spurious autosave a chance to fire if
    // the load-vs-edit guard were broken — see pages/ballot/[weight].vue's
    // nextTick() comment for the bug this pins.
    await new Promise((r) => setTimeout(r, 600))
    expect(patchBody).toBeUndefined()
  })
})
