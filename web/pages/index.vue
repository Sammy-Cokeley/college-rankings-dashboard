<script setup lang="ts">
import type { RankingsOverview, SeasonSeries } from '~/types/rankings'
import { WEIGHT_CLASSES } from '~/utils/weights'
import { movement } from '~/utils/movement'
import { chartScales, seriesSegments, polylinePoints, type ChartBox } from '~/utils/chart'

// Non-essential fetches: the hero (headline, subhead, weight CTAs) is the
// point of this page and must never wait on — or break because of — either
// of these. Deliberately NOT awaited (unlike the rest of the codebase's
// blocking `await useFetch`): lazy:true plus no await means the page's own
// async setup doesn't wait on either request, so the hero paints
// immediately and these sections fill in reactively once data lands. No
// error is ever rethrown, only used to hide the section it feeds.
const { data: overview } = useFetch<RankingsOverview>('/api/rankings?source=flowrestling', {
  lazy: true,
})
const { data: heroSeries } = useFetch<SeasonSeries>('/api/rankings/125/series?source=flowrestling', {
  lazy: true,
})

interface Mover {
  weight: number
  rank: number
  name: string
  school: string | null
  delta: number
}

const movers = computed<Mover[]>(() => {
  const rows = (overview.value?.weights ?? []).flatMap((w) =>
    w.entries.map((e) => ({ weight: w.weight, entry: e, m: movement(e.rank, e.prevRank) })),
  )
  return rows
    .filter((r) => r.m.kind === 'up')
    .sort((a, b) => b.m.delta - a.m.delta)
    .slice(0, 5)
    .map((r) => ({
      weight: r.weight,
      rank: r.entry.rank,
      name: r.entry.name,
      school: r.entry.school,
      delta: r.m.delta,
    }))
})

// --- decorative "rank lines" hero chart --------------------------------
// Ambient, non-interactive — same pure geometry TrajectoryChart.vue uses,
// stripped of hover/click/legend. Ties to the documented "rank lines" brand
// motif (docs/decisions.md) without doing the site rename, which is gated on
// a separate domain-registration step.

const heroBox: ChartBox = { width: 860, height: 360, padTop: 10, padRight: 10, padBottom: 10, padLeft: 10 }

const heroMaxRank = computed(() =>
  Math.max(1, ...(heroSeries.value?.series.flatMap((s) => s.points.map((p) => p.rank)) ?? [])),
)
const heroScales = computed(() =>
  chartScales(heroSeries.value?.weeks.length ?? 1, heroMaxRank.value, heroBox),
)
// A single muted tone, not the interactive chart's per-wrestler palette —
// this is ambient texture behind the hero copy, not something to read.
const heroLines = computed(() =>
  (heroSeries.value?.series ?? []).map((s) => ({
    key: s.wrestlerId,
    segments: seriesSegments(s.points).filter((seg) => seg.length > 1),
  })),
)
const poly = (seg: { week: number; rank: number }[]) => polylinePoints(seg, heroScales.value)

useSeoMeta({
  title: 'NCAA DI Wrestling Rankings',
  description:
    'Rank the wrestlers you know at any weight class — top 10 or deeper, up to 33 — then compare your ballot against FloWrestling and the Fan Poll, updated every week.',
  ogTitle: 'NCAA DI Wrestling Rankings',
  ogDescription: 'Rank the room. Compare with everyone else’s.',
  ogType: 'website',
  twitterCard: 'summary',
})
</script>

<template>
  <div class="landing">
    <section class="hero">
      <svg
        v-if="heroLines.length"
        class="hero-chart"
        aria-hidden="true"
        :viewBox.attr="`0 0 ${heroBox.width} ${heroBox.height}`"
        preserveAspectRatio="none"
      >
        <g v-for="line in heroLines" :key="line.key" class="hl">
          <polyline
            v-for="(seg, i) in line.segments"
            :key="i"
            :points.attr="poly(seg)"
          />
        </g>
      </svg>
      <div class="hero-scrim" aria-hidden="true" />

      <div class="hero-content">
        <p class="kicker">NCAA DI Wrestling / Fan Poll</p>
        <h1>Rank the room.<br>Compare with everyone else&rsquo;s.</h1>
        <p class="subhead">
          Rank the wrestlers you know at any weight class — a top 10 is a solid ballot, go
          deeper if you want — then see how it stacks up against FloWrestling and the crowd.
        </p>

        <div class="cta">
          <p class="cta-label">Build your ballot</p>
          <nav class="weight-grid" aria-label="Pick a weight class to build your ballot">
            <NuxtLink v-for="w in WEIGHT_CLASSES" :key="w" :to="`/ballot/${w}`" class="weight-btn">
              {{ w }}
            </NuxtLink>
          </nav>
        </div>
      </div>
    </section>

    <section class="movers board">
      <h2>Biggest movers this week</h2>
      <ol v-if="movers.length" class="movers-list">
        <li v-for="m in movers" :key="`${m.weight}-${m.rank}-${m.name}`">
          <NuxtLink :to="`/${m.weight}`" class="movers-row">
            <span class="mv up">&#9650;{{ m.delta }}</span>
            <span class="name">{{ m.name }}</span>
            <span class="school">{{ m.school }}</span>
            <span class="weight">{{ m.weight }} lbs</span>
          </NuxtLink>
        </li>
      </ol>
      <p v-else class="empty">Movers will show up here once this week's rankings are in.</p>
      <NuxtLink to="/rankings" class="see-all">See full rankings &rarr;</NuxtLink>
    </section>

    <section class="explainer">
      <h2>How the Fan Poll works</h2>
      <ol class="steps">
        <li>
          <span class="step-label">Rank</span>
          <p>Search wrestlers and order as many as you know — top 10 is plenty, up to 33 if you want.</p>
        </li>
        <li>
          <span class="step-label">Combine</span>
          <p>Every fan's ballot rolls up into one ranking per weight class.</p>
        </li>
        <li>
          <span class="step-label">Compare</span>
          <p>See the Fan Poll next to FloWrestling, side by side, every week.</p>
        </li>
      </ol>
    </section>
  </div>
</template>

<style scoped>
.landing {
  padding-bottom: 2rem;
}

/* --- hero -------------------------------------------------------------- */

.hero {
  position: relative;
  border-radius: 0.75rem;
  overflow: hidden;
  background: var(--surface);
  border: 1px solid var(--line);
  margin-top: 1.5rem;
  padding: 3rem 2rem;
}

.hero-chart {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.hero-chart polyline {
  fill: none;
  stroke: var(--muted);
  stroke-width: 1.5;
  stroke-linejoin: round;
  stroke-linecap: round;
  opacity: 0.15;
}

/* Guarantees the copy reads clearly regardless of how busy the ambient
   chart gets — a solid scrim over the text column, fading into the chart. */
.hero-scrim {
  position: absolute;
  inset: 0;
  width: 60%;
  background: linear-gradient(90deg, var(--surface) 55%, transparent 100%);
  pointer-events: none;
}

.hero-content {
  position: relative;
  z-index: 1;
  max-width: 34rem;
}

.kicker {
  font-family: var(--font-mono);
  font-size: 0.78rem;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--accent);
  margin: 0 0 0.75rem;
}

.hero h1 {
  margin: 0;
  font-size: 2.6rem;
  font-weight: 700;
  line-height: 1.12;
  letter-spacing: 0.01em;
}

.subhead {
  color: var(--muted);
  font-size: 1rem;
  margin: 1rem 0 0;
  max-width: 30rem;
}

.cta {
  margin-top: 2rem;
}

.cta-label {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--muted);
  margin: 0 0 0.6rem;
}

.weight-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.weight-btn {
  font-family: var(--font-mono);
  font-weight: 600;
  font-size: 0.95rem;
  color: var(--accent-ink);
  background: var(--chip);
  border-radius: 0.4rem;
  padding: 0.5rem 0.9rem;
  text-decoration: none;
  transition: opacity 0.15s;
}

.weight-btn:hover {
  opacity: 0.85;
}

/* --- movers -------------------------------------------------------------- */

.movers {
  margin-top: 1.5rem;
  padding: 1.25rem 1.5rem 1.5rem;
}

.movers h2,
.explainer h2 {
  margin: 0 0 1rem;
  font-size: 0.8rem;
  font-family: var(--font-mono);
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--muted);
}

.movers-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.movers-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 0.25rem;
  border-radius: 0.35rem;
  text-decoration: none;
  color: inherit;
}

.movers-row:hover {
  background: var(--surface-hover);
}

.movers-row .mv {
  font-family: var(--font-mono);
  font-weight: 600;
  color: var(--up);
  width: 2.5rem;
}

.movers-row .name {
  font-weight: 600;
  flex: 1;
}

.movers-row .school,
.movers-row .weight {
  color: var(--muted);
  font-size: 0.85rem;
}

.movers .empty {
  color: var(--muted);
  margin: 0;
}

.see-all {
  display: inline-block;
  margin-top: 1rem;
  color: var(--accent);
  font-size: 0.85rem;
  font-weight: 600;
  text-decoration: none;
}

.see-all:hover {
  text-decoration: underline;
}

/* --- explainer ----------------------------------------------------------- */

.explainer {
  margin-top: 1.5rem;
}

.steps {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 1rem;
}

.steps li {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 0.5rem;
  padding: 1rem 1.1rem;
}

.step-label {
  display: block;
  font-family: var(--font-mono);
  font-weight: 700;
  font-size: 0.8rem;
  letter-spacing: 0.1em;
  color: var(--accent);
  margin-bottom: 0.4rem;
}

.steps p {
  margin: 0;
  color: var(--muted);
  font-size: 0.9rem;
}

@media (max-width: 40rem) {
  .hero h1 {
    font-size: 2rem;
  }
}
</style>
