// Shared response shapes for the ballot-builder feature (Fan Poll, Phase 3).
// Unlike types/rankings.ts, these describe the FIRST write path in this
// app — see server/utils/ballot-queries.ts.

export interface WrestlerOption {
  wrestlerId: number
  name: string // wrestlers.full_name (canonical, not a raw published string)
  school: string | null
  weightClass: number | null // this wrestler's current roster-assigned weight; may differ from the ballot being built
}

export interface BallotEntry {
  rank: number
  wrestlerId: number
  name: string
  school: string | null
}

export interface Ballot {
  weightClass: number
  season: number
  updatedAt: string | null // null when the user has no ballot yet at this weight
  entries: BallotEntry[]
}

// One past Submit — the append-only history /profile's "Previous ballots"
// list reads (server/utils/ballot-queries.ts getBallotHistory). Distinct
// from Ballot: this is a point-in-time snapshot, never updated after the
// fact, unlike the live/rolling ballot the builder page edits.
export interface BallotSubmission {
  id: number
  weightClass: number
  submittedAt: string
  entries: BallotEntry[]
}
