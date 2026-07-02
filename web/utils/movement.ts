export type MovementKind = 'new' | 'up' | 'down' | 'even'

export interface MovementDisplay {
  kind: MovementKind
  delta: number // absolute rank change; 0 for 'new' and 'even'
}

// movement classifies a week-over-week rank change for display. prevRank is
// null on a wrestler's first appearance at this weight (schema.md §5) — that
// includes a mid-season move from another weight class, by design.
export function movement(rank: number, prevRank: number | null): MovementDisplay {
  if (prevRank === null) return { kind: 'new', delta: 0 }
  const delta = prevRank - rank
  if (delta > 0) return { kind: 'up', delta }
  if (delta < 0) return { kind: 'down', delta: -delta }
  return { kind: 'even', delta: 0 }
}
