export default defineNuxtConfig({
  compatibilityDate: '2026-07-01',
  css: ['~/assets/css/main.css'],
  modules: ['nuxt-auth-utils'],
  runtimeConfig: {
    // Postgres connection string. Defaults to $DATABASE_URL (shared with the
    // Go pipeline — see .env.example) so one var configures both apps;
    // NUXT_DATABASE_URL still overrides it per Nuxt's usual runtimeConfig
    // convention if the two ever need to diverge.
    databaseUrl: process.env.DATABASE_URL || '',
    // nuxt-auth-utils reads NUXT_SESSION_PASSWORD itself; see .env.example.
  },
})
