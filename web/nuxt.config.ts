export default defineNuxtConfig({
  compatibilityDate: '2026-07-01',
  css: ['~/assets/css/main.css'],
  runtimeConfig: {
    // Path to the pipeline-built SQLite DB, relative to the server process cwd.
    // Override with NUXT_DB_PATH. The web app never writes (v0 invariant).
    dbPath: '../pipeline/rankings.db',
  },
})
