<script setup lang="ts">
import { WEIGHT_CLASSES } from './utils/weights'
</script>

<template>
  <div class="site">
    <header class="site-header">
      <NuxtLink to="/" class="site-title">NCAA DI Wrestling Rankings</NuxtLink>
      <nav class="weight-nav" aria-label="Weight classes">
        <NuxtLink v-for="w in WEIGHT_CLASSES" :key="w" :to="`/${w}`">{{ w }}</NuxtLink>
      </nav>
    </header>
    <main class="site-main">
      <NuxtPage />
    </main>
    <footer class="site-footer">
      Rankings as published by FloWrestling. Movement derived week-over-week.
    </footer>
  </div>
</template>

<style>
:root {
  --ink: #1a1a1a;
  --muted: #6b7280;
  --line: #e5e7eb;
  --accent: #1d4ed8;
  --up: #15803d;
  --down: #b91c1c;
  --bg: #ffffff;
  --bg-alt: #f9fafb;
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  color: var(--ink);
  background: var(--bg);
  font-family: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  line-height: 1.5;
}

a {
  color: var(--accent);
  text-decoration: none;
}

a:hover {
  text-decoration: underline;
}

.site {
  max-width: 60rem;
  margin: 0 auto;
  padding: 0 1rem;
}

.site-header {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.75rem 1.5rem;
  padding: 1rem 0;
  border-bottom: 2px solid var(--ink);
}

.site-title {
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--ink);
}

.weight-nav {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 0.75rem;
  font-variant-numeric: tabular-nums;
}

.weight-nav a.router-link-active {
  font-weight: 700;
  text-decoration: underline;
}

.site-main {
  padding: 1.5rem 0 3rem;
}

.site-footer {
  padding: 1rem 0 2rem;
  border-top: 1px solid var(--line);
  color: var(--muted);
  font-size: 0.85rem;
}

/* Rankings tables (dashboard + weight pages) */
.rank-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.95rem;
}

.rank-table th {
  text-align: left;
  padding: 0.4rem 0.6rem;
  border-bottom: 2px solid var(--ink);
  white-space: nowrap;
}

.rank-table td {
  padding: 0.4rem 0.6rem;
  border-bottom: 1px solid var(--line);
}

.rank-table tbody tr:nth-child(even) {
  background: var(--bg-alt);
}

.rank-table .num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.rank-table .rank {
  font-weight: 700;
}
</style>
