import { z } from 'zod'
import { useDb } from '../../utils/db'
import { checkRateLimit, requestKey } from '../../utils/rate-limit'

const loginSchema = z.object({
  email: z.string().trim().toLowerCase().email(),
  password: z.string().min(1),
})

// POST /api/auth/login — rate-limited by IP, generous enough for a genuine
// user (10/hr) while blunting scripted credential-stuffing without a captcha.
export default defineEventHandler(async (event) => {
  if (!checkRateLimit(requestKey(event, 'login'), 10, 60 * 60 * 1000)) {
    throw createError({ statusCode: 429, statusMessage: 'Too many login attempts. Try again later.' })
  }

  const parsed = loginSchema.safeParse(await readBody(event))
  if (!parsed.success) {
    throw createError({ statusCode: 400, statusMessage: 'Invalid email or password' })
  }

  const db = useDb()
  const rows = await db<
    Array<{
      id: number
      email: string
      passwordHash: string
      displayName: string | null
      sessionEpoch: number
    }>
  >`
    SELECT id, email, password_hash AS "passwordHash", display_name AS "displayName",
           session_epoch AS "sessionEpoch"
    FROM users WHERE email = ${parsed.data.email}`
  const user = rows[0]

  // Same error for "no such user" and "wrong password" — never reveal which.
  const invalidCredentials = () =>
    createError({ statusCode: 401, statusMessage: 'Invalid email or password' })
  if (!user) throw invalidCredentials()
  if (!(await verifyPassword(user.passwordHash, parsed.data.password))) throw invalidCredentials()

  await setUserSession(event, {
    user: { id: user.id, email: user.email, displayName: user.displayName },
    epoch: user.sessionEpoch,
  })

  return { id: user.id, email: user.email, displayName: user.displayName }
})
