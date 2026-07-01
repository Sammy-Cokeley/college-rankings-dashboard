import { existsSync } from 'node:fs'
import { resolve } from 'node:path'
import Database from 'better-sqlite3'
import type { Db } from './queries'

let db: Db | null = null

// useDb returns the shared read-only connection to the pipeline-built SQLite
// DB. The web app never writes (v0 invariant); the pipeline is the single
// weekly writer and WAL mode keeps our reads from ever blocking it.
export function useDb(): Db {
  if (db) return db
  const path = resolve(process.cwd(), useRuntimeConfig().dbPath)
  if (!existsSync(path)) {
    throw new Error(
      `rankings DB not found at ${path}. Build it first (npm run db:build, ` +
        `or see web/README.md), or point NUXT_DB_PATH at an existing DB.`,
    )
  }
  db = new Database(path, { readonly: true, fileMustExist: true })
  return db
}
