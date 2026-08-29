// POST /api/auth/logout — clears the session cookie. No DB write: there's no
// server-side session row to delete (schema.md §9).
export default defineEventHandler(async (event) => {
  await clearUserSession(event)
  return { ok: true }
})
