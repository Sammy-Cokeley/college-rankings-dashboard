import postgres from 'postgres'
import type { Db } from './queries'

let db: Db | null = null

// useDb returns the shared Postgres connection. Unlike the SQLite-era
// connection this replaces, there's no client-side readonly flag — this app
// still only issues SELECTs (no write routes exist yet), but that's no longer
// enforced here. If/when a write path is added (e.g. the community/ballots
// feature), enforcing "web never writes rankings data" for real means a
// Postgres role with SELECT-only grants on the rankings tables, not a client
// option.
export function useDb(): Db {
  if (db) return db
  const url = useRuntimeConfig().databaseUrl
  if (!url) {
    throw new Error(
      'DATABASE_URL not set. Point it at a Postgres instance (see .env.example), ' +
        'or build one locally (docker compose up -d && npm run db:build).',
    )
  }
  db = postgres(url)
  return db
}
