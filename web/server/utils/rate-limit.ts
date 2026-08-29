// In-memory sliding-window rate limiter. Good enough for a single,
// non-clustered Nitro process (the project's actual deploy shape) — if the
// host ever runs multiple instances behind a load balancer, each instance's
// counters go out of sync and this stops being globally accurate. Revisit
// with a shared store (Redis, or a Postgres table) if that ever happens.
const hits = new Map<string, number[]>()

// checkRateLimit records one hit for key and reports whether the caller is
// still within {max} hits per {windowMs}. Old hits outside the window are
// pruned on every call, so the map never grows unbounded for a key that's
// gone quiet.
export function checkRateLimit(key: string, max: number, windowMs: number): boolean {
  const now = Date.now()
  const cutoff = now - windowMs
  const recent = (hits.get(key) ?? []).filter((t) => t > cutoff)
  recent.push(now)
  hits.set(key, recent)
  return recent.length <= max
}

// requestKey builds a rate-limit key from the client IP and an action name
// (e.g. "signup", "login") so limits are scoped per action, not shared
// across every endpoint one IP happens to hit.
export function requestKey(event: Parameters<typeof getRequestIP>[0], action: string): string {
  const ip = getRequestIP(event, { xForwardedFor: true }) ?? 'unknown'
  return `${action}:${ip}`
}
