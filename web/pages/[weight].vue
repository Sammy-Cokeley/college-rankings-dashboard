<script setup lang="ts">
import type { SeasonSeries, WeightRankings } from '~/types/rankings'
import { isWeightClass } from '~/utils/weights'

definePageMeta({
  validate: (route) => isWeightClass(Number(route.params.weight)),
})

const route = useRoute()
const router = useRouter()
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

// The bump chart is season-wide, independent of the selected edition. Its
// failure is non-fatal: the rankings table still renders without it.
const seriesUrl = computed(() => `/api/rankings/${weight.value}/series`)
const { data: seriesData } = await useFetch<SeasonSeries>(seriesUrl)

const edition = computed(() => data.value!.edition)
const dates = computed(() => data.value!.dates)

const prevDate = computed(() => dates.value[edition.value.week - 2]?.date ?? null)
const nextDate = computed(() => dates.value[edition.value.week]?.date ?? null)

// --- selection (shareable via ?sel=) ---------------------------------------

const selected = computed<number[]>(() => {
  const raw = route.query.sel
  if (typeof raw !== 'string' || raw === '') return []
  const known = new Set((seriesData.value?.series ?? []).map((s) => s.wrestlerId))
  return [...new Set(raw.split(',').map(Number))].filter((id) => known.has(id))
})

function setSelected(ids: number[]) {
  const { sel: _drop, ...rest } = route.query
  router.replace({ query: ids.length ? { ...rest, sel: ids.join(',') } : rest })
}

function toggleWrestler(wrestlerId: number | null) {
  if (wrestlerId === null) return // unresolved entries have no line to pin
  const current = selected.value
  setSelected(
    current.includes(wrestlerId)
      ? current.filter((id) => id !== wrestlerId)
      : [...current, wrestlerId],
  )
}

// Toggles the whole school group (current edition's rows): if every wrestler
// from the school is already selected, deselect them; otherwise add the rest.
function toggleSchool(school: string | null) {
  if (!school) return
  const ids = edition.value.entries
    .filter((r) => r.school === school && r.wrestlerId !== null)
    .map((r) => r.wrestlerId!)
  if (ids.length === 0) return
  const current = new Set(selected.value)
  const allIn = ids.every((id) => current.has(id))
  if (allIn) {
    setSelected(selected.value.filter((id) => !ids.includes(id)))
  } else {
    setSelected([...selected.value, ...ids.filter((id) => !current.has(id))])
  }
}

function rowSelectionIndex(wrestlerId: number | null): number {
  return wrestlerId === null ? -1 : selected.value.indexOf(wrestlerId)
}

// --- table fold (top 10 visible, rest behind "see more") --------------------

const FOLD_RANK = 10

// Cut by rank, not row count, so a tie at the fold never gets half-hidden.
// Hidden rows stay in the DOM (CSS display:none) — SSR HTML keeps all names.
const foldedCount = computed(() => edition.value.entries.filter((r) => r.rank > FOLD_RANK).length)

function selectionBelowFold(): boolean {
  const sel = new Set(selected.value)
  return edition.value.entries.some((r) => r.wrestlerId !== null && sel.has(r.wrestlerId) && r.rank > FOLD_RANK)
}

// Initialized identically on server and client (both derive from the same
// route + payload), so a ?sel= deep link to a below-fold wrestler SSRs the
// table already expanded — no hydration mismatch.
const expanded = ref(selectionBelowFold())

// Pinning a hidden wrestler later (e.g. clicking their line in the chart)
// reveals their row; collapsing again is always manual.
watch(selected, () => {
  if (!expanded.value && selectionBelowFold()) expanded.value = true
})

// Fresh weight page = fresh fold.
watch(weight, () => {
  expanded.value = selectionBelowFold()
})

// --- edition navigation ------------------------------------------------------

function toEdition(date: string | null) {
  if (!date) return
  const query: Record<string, string> = {}
  if (selected.value.length) query.sel = selected.value.join(',')
  if (date !== dates.value[dates.value.length - 1]?.date) query.date = date
  navigateTo({ query })
}

const seasonLabel = computed(() => `${edition.value.season - 1}-${String(edition.value.season).slice(2)}`)

const pageTitle = computed(
  () =>
    `${weight.value} lbs — NCAA DI Wrestling Rankings (Week ${edition.value.week}, ${edition.value.date})`,
)
const pageDescription = computed(
  () =>
    `${data.value!.source} NCAA DI wrestling rankings at ${weight.value} lbs, week ${edition.value.week} of the ${seasonLabel.value} season, with week-over-week movement and season trajectories.`,
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
          <tr
            v-for="row in edition.entries"
            :key="`${row.rank}-${row.name}`"
            class="selectable"
            :class="{
              selected: rowSelectionIndex(row.wrestlerId) >= 0,
              folded: !expanded && row.rank > FOLD_RANK,
            }"
            :style="rowSelectionIndex(row.wrestlerId) >= 0
              ? { '--row-accent': `var(--chart-${rowSelectionIndex(row.wrestlerId) % 6})` }
              : undefined"
            @click="toggleWrestler(row.wrestlerId)"
          >
            <td class="num rank" :class="{ top: row.rank <= 3 }">{{ row.rank }}</td>
            <td class="name">{{ row.name }}</td>
            <td class="school school-toggle" @click.stop="toggleSchool(row.school)">{{ row.school }}</td>
            <td class="grade">{{ row.grade }}</td>
            <td class="num"><MovementBadge :rank="row.rank" :prev-rank="row.prevRank" /></td>
          </tr>
        </tbody>
      </table>
      <div v-if="foldedCount > 0" class="fold-toggle">
        <button @click="expanded = !expanded">
          {{ expanded ? `Show top ${FOLD_RANK} ▴` : `See all ${edition.entries.length} ranked ▾` }}
        </button>
      </div>
    </div>

    <section v-if="seriesData && seriesData.series.length" class="chart-panel board">
      <header class="chart-head">
        <h2>Season trajectory</h2>
        <p class="hint">
          Hover a line to identify it. Click a line or a table row to pin a wrestler; click a
          school to pin the whole room.
        </p>
      </header>
      <TrajectoryChart
        :weeks="seriesData.weeks"
        :series="seriesData.series"
        :selected="selected"
        @toggle="toggleWrestler"
      />
    </section>
  </div>
</template>
