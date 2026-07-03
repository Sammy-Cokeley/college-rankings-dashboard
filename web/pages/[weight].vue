<script setup lang="ts">
import type { WeightRankings } from '~/types/rankings'
import { isWeightClass } from '~/utils/weights'

definePageMeta({
  validate: (route) => isWeightClass(Number(route.params.weight)),
})

const route = useRoute()
// Reactive: the router reuses this component instance when navigating between
// weight pages, so a plain const would freeze the first weight visited.
const weight = computed(() => Number(route.params.weight))

const url = computed(() => {
  const date = route.query.date
  const query = typeof date === 'string' ? `?date=${encodeURIComponent(date)}` : ''
  return `/api/rankings/${weight.value}${query}`
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

const pageTitle = computed(
  () =>
    `${weight.value} lbs — NCAA DI Wrestling Rankings (Week ${edition.value.week}, ${edition.value.date})`,
)
const pageDescription = computed(
  () =>
    `${data.value!.source} NCAA DI wrestling rankings at ${weight.value} lbs, week ${edition.value.week} of the ${seasonLabel.value} season, with week-over-week movement.`,
)

useSeoMeta({
  title: pageTitle,
  description: pageDescription,
  ogTitle: pageTitle,
  ogDescription: pageDescription,
  ogType: 'website',
  twitterCard: 'summary',
})
</script>

<template>
  <div v-if="data">
    <div class="pagehead">
      <div>
        <h1>{{ weight }}<small>LBS</small></h1>
        <p class="sub">
          WK {{ edition.week }} / {{ edition.date }} / {{ data.source }} / {{ seasonLabel }}
        </p>
      </div>
      <nav class="controls" aria-label="Editions">
        <button :disabled="!prevDate" aria-label="Previous week" @click="toEdition(prevDate)">←</button>
        <select
          aria-label="Week"
          :value="edition.date"
          @change="toEdition(($event.target as HTMLSelectElement).value)"
        >
          <option v-for="d in dates" :key="d.date" :value="d.date">
            WK {{ d.week }} — {{ d.date }}
          </option>
        </select>
        <button :disabled="!nextDate" aria-label="Next week" @click="toEdition(nextDate)">→</button>
      </nav>
    </div>

    <div class="board">
      <table>
        <thead>
          <tr>
            <th class="num">RK</th>
            <th>Wrestler</th>
            <th>School</th>
            <th>YR</th>
            <th class="num">Move</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in edition.entries" :key="`${row.rank}-${row.name}`">
            <td class="num rank" :class="{ top: row.rank <= 3 }">{{ row.rank }}</td>
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
