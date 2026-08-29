import { z } from 'zod'
import { useDb } from '../../utils/db'
import { checkRateLimit, requestKey } from '../../utils/rate-limit'

const signupSchema = z.object({
  email: z.string().trim().toLowerCase().email(),
  password: z.string().min(8).max(200),
  displayName: z.string().trim().min(1).max(80).optional(),
})

// POST /api/auth/signup — open, self-service (docs/decisions.md: no
// invite-only gate). Rate-limited by IP; email/password validated at this
// boundary before anything touches the DB.
export default defineEventHandler(async (event) => {
  if (!checkRateLimit(requestKey(event, 'signup'), 5, 60 * 60 * 1000)) {
    throw createError({ statusCode: 429, statusMessage: 'Too many signup attempts. Try again later.' })
  }

  const parsed = signupSchema.safeParse(await readBody(event))
  if (!parsed.success) {
    throw createError({ statusCode: 400, statusMessage: parsed.error.issues[0]?.message ?? 'Invalid request' })
  }
  const { email, password, displayName } = parsed.data

  const db = useDb()
  const existing = await db<{ id: number }[]>`SELECT id FROM users WHERE email = ${email}`
  if (existing.length > 0) {
    throw createError({ statusCode: 409, statusMessage: 'An account with that email already exists' })
  }

  const passwordHash = await hashPassword(password)

  let user: { id: number; email: string; displayName: string | null }
  try {
    const rows = await db<Array<{ id: number; email: string; displayName: string | null }>>`
      INSERT INTO users (email, password_hash, display_name, created_at)
      VALUES (${email}, ${passwordHash}, ${displayName ?? null}, ${new Date().toISOString()})
      RETURNING id, email, display_name AS "displayName"`
    user = rows[0]!
  } catch (error: unknown) {
    // A concurrent signup for the same email can slip past the existence
    // check above and only fail here, at the DB's UNIQUE constraint
    // (Postgres unique_violation = 23505) — same clean 409, not a raw DB error.
    if (error && typeof error === 'object' && 'code' in error && error.code === '23505') {
      throw createError({ statusCode: 409, statusMessage: 'An account with that email already exists' })
    }
    throw error
  }

  await setUserSession(event, {
    user: { id: user.id, email: user.email, displayName: user.displayName },
    epoch: 1,
  })

  return { id: user.id, email: user.email, displayName: user.displayName }
})
