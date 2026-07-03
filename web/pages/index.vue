<script setup lang="ts">
import type { RankingsOverview } from '~/types/rankings'
import { movement } from '~/utils/movement'
import { WEIGHT_CLASSES } from '~/utils/weights'

const { data, error } = await useFetch<RankingsOverview>('/api/rankings')

if (error.value) {
  throw createError({
    statusCode: error.value.statusCode ?? 500,
    statusMessage: error.value.statusMessage ?? 'Failed to load rankings',
  })
}

interface DashboardRow {
  weight: number
  rank: number
  name: string
  school: string | null
  grade: string | null
  prevRank: number | null
  delta: number | null // signed; null = first appearance (sorts last)
}

const allRows = computed<DashboardRow[]>(() =>
  (data.value?.weights ?? []).flatMap((w) =>
    w.entries.map((e) => {
      const m = movement(e.rank, e.prevRank)
      return {
        weight: w.weight,
        rank: e.rank,
        name: e.name,
        school: e.school,
        grade: e.grade,
        prevRank: e.prevRank,
        delta: m.kind === 'new' ? null : m.kind === 'down' ? -m.delta : m.delta,
      }
    }),
  ),
)

const weightFilter = ref<number | 'all'>('all')
const search = ref('')

type SortKey = 'weight' | 'rank' | 'name' | 'school' | 'delta'
const sortKey = ref<SortKey>('weight')
const sortAsc = ref(true)

function sortBy(key: SortKey) {
  if (sortKey.value === key) {
    sortAsc.value = !sortAsc.value
  } else {
    sortKey.value = key
    // Movement reads best descending (big risers first); the rest ascending.
    sortAsc.value = key !== 'delta'
  }
}

function compare(a: DashboardRow, b: DashboardRow, key: SortKey): number {
  if (key === 'name' || key === 'school') {
    return (a[key] ?? '').localeCompare(b[key] ?? '')
  }
  if (key === 'delta') {
    // Null deltas (NEW) always sort after real movement, either direction.
    if (a.delta === null && b.delta === null) return 0
    if (a.delta === null) return 1
    if (b.delta === null) return -1
    return sortAsc.value ? a.delta - b.delta : b.delta - a.delta
  }
  return a[key] - b[key]
}

const rows = computed(() => {
  const needle = search.value.trim().toLowerCase()
  const filtered = allRows.value.filter((r) => {
    if (weightFilter.value !== 'all' && r.weight !== weightFilter.value) return false
    if (!needle) return true
    return (
      r.name.toLowerCase().includes(needle) || (r.school ?? '').toLowerCase().includes(needle)
    )
  })
  const key = sortKey.value
  const dir = key === 'delta' ? 1 : sortAsc.value ? 1 : -1
  return [...filtered].sort((a, b) => {
    const primary = compare(a, b, key) * dir
    if (primary !== 0) return primary
    // Stable, readable fallback: weight then published rank then name (ties).
    return a.weight - b.weight || a.rank - b.rank || a.name.localeCompare(b.name)
  })
})

function arrow(key: SortKey) {
  if (sortKey.value !== key) return ''
  return sortAsc.value ? ' ↑' : ' ↓'
}

const seasonLabel = computed(() =>
  data.value ? `${data.value.season - 1}-${String(data.value.season).slice(2)}` : '',
)

const pageDescription = computed(
  () =>
    `Latest ${data.value?.source ?? ''} NCAA DI wrestling rankings for all ten weight classes, ${seasonLabel.value} season, with week-over-week movement.`,
)

useSeoMeta({
  title: 'NCAA DI Wrestling Rankings — All Weights',
  description: pageDescription,
  ogTitle: 'NCAA DI Wrestling Rankings — All Weights',
  ogDescription: pageDescription,
  ogType: 'website',
  twitterCard: 'summary',
})
</script>

<template>
  <div v-if="data">
    <div class="pagehead">
      <div>
        <h1>All Weights</h1>
        <p class="sub">LATEST EDITIONS / {{ data.source }} / {{ seasonLabel }}</p>
      </div>
      <div class="controls">
        <select v-model="weightFilter" aria-label="Filter by weight class">
          <option value="all">Weight: All</option>
          <option v-for="w in WEIGHT_CLASSES" :key="w" :value="w">{{ w }}</option>
        </select>
        <input
          v-model="search"
          type="search"
          placeholder="Search wrestler or school…"
          aria-label="Search wrestler or school"
        >
        <span class="count">{{ rows.length }} RANKED</span>
      </div>
    </div>

    <div class="board">
      <table>
        <thead>
          <tr>
            <th class="num sortable" @click="sortBy('weight')">WT{{ arrow('weight') }}</th>
            <th class="num sortable" @click="sortBy('rank')">RK{{ arrow('rank') }}</th>
            <th class="sortable" @click="sortBy('name')">Wrestler{{ arrow('name') }}</th>
            <th class="sortable" @click="sortBy('school')">School{{ arrow('school') }}</th>
            <th>YR</th>
            <th class="num sortable" @click="sortBy('delta')">Move{{ arrow('delta') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="`${row.weight}-${row.rank}-${row.name}`">
            <td class="num weight"><NuxtLink :to="`/${row.weight}`">{{ row.weight }}</NuxtLink></td>
            <td class="num rank">{{ row.rank }}</td>
            <td class="name">{{ row.name }}</td>
            <td class="school">{{ row.school }}</td>
            <td class="grade">{{ row.grade }}</td>
            <td class="num"><MovementBadge :rank="row.rank" :prev-rank="row.prevRank" /></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.controls input[type='search'] {
  min-width: 14rem;
}
</style>
