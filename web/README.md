# web/ — Nuxt 3 front end (deferred)

Placeholder. The Nuxt 3 (Vue 3, SSR) app is **not** initialized yet.

Per `docs/decisions.md`, the web app reads the shared SQLite database directly
via Nitro server routes (no separate API layer in v0). It is intentionally held
until the pipeline and store layer are proven and there is data to render.

When ready, initialize Nuxt 3 here (`npx nuxi init .` into this directory) — the
empty `web/` slot in the monorepo is reserved for exactly that.
