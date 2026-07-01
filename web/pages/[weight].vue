<script setup lang="ts">
import type { WeightRankings } from '~/types/rankings'
import { isWeightClass } from '~/utils/weights'

definePageMeta({
  validate: (route) => isWeightClass(Number(route.params.weight)),
})

const route = useRoute()
const weight = Number(route.params.weight)

const url = computed(() => {
  const date = route.query.date
  const query = typeof date === 'string' ? `?date=${encodeURIComponent(date)}` : ''
  return `/api/rankings/${weight}${query}`
})

const { data, error } = await useFetch<WeightRankings>(url)

if (error.value) {
  throw createError({
    statusCode: error.value.statusCode ?? 500,
    statusMessage: error.value.statusMessage ?? 'Failed to load rankings',
  })
}

const edition = computed(() => data.value!.edition)
const dates = computed(() => data.value!.dates)

const prevDate = computed(() => dates.value[edition.value.week - 2]?.date ?? null)
const nextDate = computed(() => dates.value[edition.value.week]?.date ?? null)

function toEdition(date: string | null) {
  if (!date) return
  navigateTo({ query: date === dates.value[dates.value.length - 1]?.date ? {} : { date } })
}

const seasonLabel = computed(() => `${edition.value.season - 1}-${String(edition.value.season).slice(2)}`)

useHead(() => ({
  title: `${weight} lbs — NCAA DI Wrestling Rankings (Week ${edition.value.week}, ${edition.value.date})`,
  meta: [
    {
      name: 'description',
      content: `${data.value!.source} NCAA DI wrestling rankings at ${weight} lbs, week ${edition.value.week} of the ${seasonLabel.value} season, with week-over-week movement.`,
    },
  ],
}))
</script>

<template>
  <div v-if="data">
    <header class="edition-header">
      <h1>{{ weight }} lbs</h1>
      <p class="edition-meta">
        {{ data.source }} · {{ seasonLabel }} season · Week {{ edition.week }} ·
        published {{ edition.date }}
      </p>
      <nav class="edition-nav" aria-label="Editions">
        <button :disabled="!prevDate" @click="toEdition(prevDate)">← previous</button>
        <label>
          Week
          <select :value="edition.date" @change="toEdition(($event.target as HTMLSelectElement).value)">
            <option v-for="d in dates" :key="d.date" :value="d.date">
              {{ d.week }} — {{ d.date }}
            </option>
          </select>
        </label>
        <button :disabled="!nextDate" @click="toEdition(nextDate)">next →</button>
      </nav>
    </header>

    <table class="rank-table">
      <thead>
        <tr>
          <th class="num">Rank</th>
          <th>Wrestler</th>
          <th>School</th>
          <th>Grade</th>
          <th class="num">Move</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in edition.entries" :key="`${row.rank}-${row.name}`">
          <td class="num rank">{{ row.rank }}</td>
          <td>{{ row.name }}</td>
          <td>{{ row.school }}</td>
          <td>{{ row.grade }}</td>
          <td class="num"><MovementBadge :rank="row.rank" :prev-rank="row.prevRank" /></td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.edition-header h1 {
  margin: 0 0 0.25rem;
}

.edition-meta {
  margin: 0;
  color: var(--muted);
}

.edition-nav {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin: 1rem 0;
}

.edition-nav button {
  padding: 0.25rem 0.6rem;
  border: 1px solid var(--line);
  border-radius: 0.25rem;
  background: var(--bg);
  cursor: pointer;
}

.edition-nav button:disabled {
  color: var(--muted);
  cursor: default;
}

.edition-nav select {
  padding: 0.2rem 0.3rem;
}
</style>
