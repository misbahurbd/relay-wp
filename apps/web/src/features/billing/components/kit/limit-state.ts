/** Share of a limit at which a meter warns. */
const ALMOST_FULL = 0.8

/** How a usage figure stands against its plan limit. limit -1 is unlimited. */
export type LimitState =
  | { kind: 'unlimited' }
  | { kind: 'ok' | 'almost'; pct: number }
  | { kind: 'at' | 'over' }

export function limitState(used: number, limit: number): LimitState {
  if (limit < 0) return { kind: 'unlimited' }
  if (used > limit) return { kind: 'over' }
  if (used === limit) return { kind: 'at' }
  const share = used / limit
  return { kind: share >= ALMOST_FULL ? 'almost' : 'ok', pct: Math.floor(share * 100) }
}
