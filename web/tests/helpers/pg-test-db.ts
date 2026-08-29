import { readdirSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import postgres from 'postgres'
import type { Db } from '../../server/utils/queries'

const migrationsDir = resolve(dirname(fileURLToPath(import.meta.url)), '../../../db/migrations')

// Every migration file, in the same filename order the Go migration runner
// applies them (pipeline/internal/store/migrate.go) — not just 0001_init.sql,
// so tests can exercise the community tables (users, roster_entries, ballots)
// added by later migrations too.
const migrationSql = readdirSync(migrationsDir)
  .filter((f) => f.endsWith('.sql'))
  .sort()
  .map((f) => readFileSync(resolve(migrationsDir, f), 'utf8'))

// createTestDb opens a connection scoped to a brand-new Postgres schema
// (mirrors pipeline/internal/store/testdb_test.go's freshDB — Postgres has no
// SQLite ":memory:" equivalent, so schema-per-test-file on a shared local
// instance is the isolation unit) with every migration already applied.
// Call cleanup() in afterAll to drop the schema and close the connection.
export async function createTestDb(): Promise<{ db: Db; cleanup: () => Promise<void> }> {
  const base = process.env.TEST_DATABASE_URL
  if (!base) {
    throw new Error('TEST_DATABASE_URL not set — start Postgres (docker compose up -d); see .env.example')
  }

  const schema = `test_${process.pid}_${Math.floor(Math.random() * 1_000_000)}`
  const url = new URL(base)
  url.searchParams.set('search_path', schema)

  const db = postgres(url.toString())
  await db.unsafe(`CREATE SCHEMA "${schema}"`)
  for (const sql of migrationSql) {
    await db.unsafe(sql)
  }

  const cleanup = async () => {
    await db.unsafe(`DROP SCHEMA IF EXISTS "${schema}" CASCADE`)
    await db.end()
  }
  return { db, cleanup }
}
