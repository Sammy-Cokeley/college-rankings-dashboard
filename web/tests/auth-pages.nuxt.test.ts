import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mockNuxtImport, mountSuspended, registerEndpoint } from '@nuxt/test-utils/runtime'
import { createError, readBody, type H3Event } from 'h3'
import SignupPage from '../pages/signup.vue'
import LoginPage from '../pages/login.vue'

// mockNuxtImport's factory is hoisted above top-level declarations (same
// vi.mock hoisting rule) — vi.hoisted() is what lets these survive that.
const { refreshSession, navigateTo } = vi.hoisted(() => ({
  refreshSession: vi.fn(),
  navigateTo: vi.fn(),
}))

mockNuxtImport('useUserSession', () => () => ({ fetch: refreshSession }))
mockNuxtImport('navigateTo', () => navigateTo)

// registerEndpoint wires a path to one handler for the whole file — calling
// it again per-test (rather than once, with a swappable handler) leaves
// stale registrations from earlier tests answering later ones. Each test
// case reassigns these instead.
let signupHandler: (event: H3Event) => unknown
registerEndpoint('/api/auth/signup', { method: 'POST', handler: (event) => signupHandler(event) })

let loginHandler: (event: H3Event) => unknown
registerEndpoint('/api/auth/login', { method: 'POST', handler: (event) => loginHandler(event) })

beforeEach(() => {
  refreshSession.mockClear()
  navigateTo.mockClear()
})

describe('signup.vue', () => {
  it('submits email/password/displayName and navigates home on success', async () => {
    let receivedBody: unknown
    signupHandler = async (event) => {
      receivedBody = await readBody(event)
      return { id: 1, email: 'new@example.com', displayName: 'New' }
    }

    const wrapper = await mountSuspended(SignupPage)
    await wrapper.find('input[type="email"]').setValue('new@example.com')
    await wrapper.find('input[type="password"]').setValue('correcthorsebattery')
    await wrapper.find('input[type="text"]').setValue('New')
    await wrapper.find('form').trigger('submit')
    await vi.waitFor(() => expect(navigateTo).toHaveBeenCalled())

    expect(receivedBody).toMatchObject({
      email: 'new@example.com',
      password: 'correcthorsebattery',
      displayName: 'New',
    })
    expect(refreshSession).toHaveBeenCalled()
    expect(navigateTo).toHaveBeenCalledWith('/')
  })

  it('shows the server error message and does not navigate on failure', async () => {
    signupHandler = () => {
      throw createError({ statusCode: 409, statusMessage: 'An account with that email already exists' })
    }

    const wrapper = await mountSuspended(SignupPage)
    await wrapper.find('input[type="email"]').setValue('dupe@example.com')
    await wrapper.find('input[type="password"]').setValue('correcthorsebattery')
    await wrapper.find('form').trigger('submit')
    await vi.waitFor(() => expect(wrapper.find('[role="alert"]').exists()).toBe(true))

    expect(wrapper.find('[role="alert"]').text()).toContain('already exists')
    expect(navigateTo).not.toHaveBeenCalled()
  })
})

describe('login.vue', () => {
  it('submits credentials and navigates home on success', async () => {
    let receivedBody: unknown
    loginHandler = async (event) => {
      receivedBody = await readBody(event)
      return { id: 1, email: 'me@example.com', displayName: null }
    }

    const wrapper = await mountSuspended(LoginPage)
    await wrapper.find('input[type="email"]').setValue('me@example.com')
    await wrapper.find('input[type="password"]').setValue('correcthorsebattery')
    await wrapper.find('form').trigger('submit')
    await vi.waitFor(() => expect(navigateTo).toHaveBeenCalled())

    expect(receivedBody).toMatchObject({ email: 'me@example.com', password: 'correcthorsebattery' })
    expect(refreshSession).toHaveBeenCalled()
    expect(navigateTo).toHaveBeenCalledWith('/')
  })

  it('shows "Invalid email or password" without revealing which was wrong', async () => {
    loginHandler = () => {
      throw createError({ statusCode: 401, statusMessage: 'Invalid email or password' })
    }

    const wrapper = await mountSuspended(LoginPage)
    await wrapper.find('input[type="email"]').setValue('me@example.com')
    await wrapper.find('input[type="password"]').setValue('wrongpassword')
    await wrapper.find('form').trigger('submit')
    await vi.waitFor(() => expect(wrapper.find('[role="alert"]').exists()).toBe(true))

    expect(wrapper.find('[role="alert"]').text()).toBe('Invalid email or password')
    expect(navigateTo).not.toHaveBeenCalled()
  })
})
