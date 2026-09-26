import { useDb } from '../../utils/db'
import { getBallotHistory } from '../../utils/ballot-queries'

// GET /api/profile/ballots — the current user's full ballot submission
// history across every weight class, newest first. The /profile page's
// only data dependency.
export default defineEventHandler(async (event) => {
  const { user } = await requireUserSession(event)
  const db = useDb()
  return getBallotHistory(db, user.id)
})
