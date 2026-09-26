<script setup lang="ts">
import type { SeasonSeries, WeightRankings } from '~/types/rankings'
import { isWeightClass } from '~/utils/weights'
import { SOURCES, DEFAULT_SOURCE } from '~/utils/sources'
import { splitName } from '~/utils/names'

definePageMeta({
  validate: (route) => isWeightClass(Number(route.params.weight)),
})

const route = useRoute()
const router = useRouter()
// Reactive: the router reuses this component instance when navigating between
// weight pages, so a plain const would freeze the first weight visited.
const weight = computed(() => Number(route.params.weight))

const sourceSlug = computed(() => {
  const raw = route.query.source
  const slug = typeof raw === 'string' ? raw : DEFAULT_SOURCE.slug
  return SOURCES.some((s) => s.slug === slug) ? slug : DEFAULT_SOURCE.slug
})

const url = computed(() => {
  const params = new URLSearchParams({ source: sourceSlug.value })
  const date = route.query.date
  if (typeof date === 'string') params.set('date', date)
  return `/api/rankings/${weight.value}?${params}`
})

const { data, error } = await useFetch<WeightRankings>(url)

// A source with no data yet (e.g. Fan Poll before enough ballots exist) is a
// normal, expected state — show it in-page with a way back to another
// source, not a hard error page.
const notYetPublished = computed(() => error.value?.statusCode === 503)

if (error.value && !notYetPublished.value) {
  throw createError({
    statusCode: error.value.statusCode ?? 500,
    statusMessage: error.value.statusMessage ?? 'Failed to load rankings',
  })
}

const currentSourceName = computed(
  () => SOURCES.find((s) => s.slug === sourceSlug.value)?.name ?? sourceSlug.value,
)

// The bump chart is season-wide, independent of the selected edition. Its
// failure is non-fatal: the rankings table still renders without it (or,
// when there's no edition at all, simply doesn't render — the template
// already guards on `seriesData.series.length`).
const seriesUrl = computed(() => `/api/rankings/${weight.value}/series?source=${sourceSlug.value}`)
const { data: seriesData } = await useFetch<SeasonSeries>(seriesUrl)

// Nullable now that a source can have no data yet (notYetPublished) — every
// consumer below must guard, not assume an edition exists.
const edition = computed(() => data.value?.edition ?? null)
const dates = computed(() => data.value?.dates ?? [])

const prevDate = computed(() =>
  edition.value ? (dates.value[edition.value.week - 2]?.date ?? null) : null,
)
const nextDate = computed(() =>
  edition.value ? (dates.value[edition.value.week]?.date ?? null) : null,
)

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
  if (!school || !edition.value) return
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
const foldedCount = computed(
  () => edition.value?.entries.filter((r) => r.rank > FOLD_RANK).length ?? 0,
)

function selectionBelowFold(): boolean {
  if (!edition.value) return false
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

// Fresh weight page = fresh fold. Keyed off the payload's weight, not the
// route param: the param flips before useFetch resolves, so a route-keyed
// reset would evaluate the fold against the OUTGOING weight's entries. If
// the series payload (and thus a still-valid ?sel=) lands after this reset,
// the selected watcher above re-expands — the two converge on fresh data.
watch(() => edition.value?.weight, () => {
  expanded.value = selectionBelowFold()
})

// --- edition navigation ------------------------------------------------------

function toEdition(date: string | null) {
  if (!date) return
  const query: Record<string, string> = {}
  if (sourceSlug.value !== DEFAULT_SOURCE.slug) query.source = sourceSlug.value
  if (selected.value.length) query.sel = selected.value.join(',')
  if (date !== dates.value[dates.value.length - 1]?.date) query.date = date
  navigateTo({ query })
}

// Switching source drops date/sel — both are meaningful only within the
// source they came from (a date valid for one source's editions is unlikely
// to exist for another's; §10 of schema.md: sources don't even share a
// season number in lockstep).
function switchSource(slug: string) {
  const query: Record<string, string> = {}
  if (slug !== DEFAULT_SOURCE.slug) query.source = slug
  navigateTo({ query })
}

// Only InterMat publishes these (schema.md §1 raw_conference/raw_record) — Flo
// and the Fan Poll rows are always null, so the columns simply don't render.
const hasConference = computed(() => edition.value?.entries.some((e) => e.conference !== null) ?? false)
const hasRecord = computed(() => edition.value?.entries.some((e) => e.record !== null) ?? false)

const seasonLabel = computed(() =>
  edition.value ? `${edition.value.season - 1}-${String(edition.value.season).slice(2)}` : '',
)

const pageTitle = computed(() =>
  edition.value
    ? `${weight.value} lbs — NCAA DI Wrestling Rankings (Week ${edition.value.week}, ${edition.value.date})`
    : `${weight.value} lbs — NCAA DI Wrestling Rankings`,
)
const pageDescription = computed(() =>
  edition.value
    ? `${data.value!.source} NCAA DI wrestling rankings at ${weight.value} lbs, week ${edition.value.week} of the ${seasonLabel.value} season, with week-over-week movement and season trajectories.`
    : `NCAA DI wrestling rankings at ${weight.value} lbs.`,
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
          <a
            v-if="data.sourceUrl"
            :href="data.sourceUrl"
            target="_blank"
            rel="noopener"
            class="source-link"
          >View on {{ data.source }} ↗</a>
        </p>
        <nav class="source-tabs" aria-label="Ranking source">
          <button
            v-for="s in SOURCES"
            :key="s.slug"
            type="button"
            :class="{ active: s.slug === sourceSlug }"
            :aria-current="s.slug === sourceSlug ? 'true' : undefined"
            @click="switchSource(s.slug)"
          >
            {{ s.name }}
          </button>
        </nav>
        <NuxtLink :to="`/ballot/${weight}`" class="ballot-cta">Build your {{ weight }} ballot →</NuxtLink>
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
      <div class="table-scroll">
        <table>
          <thead>
            <tr>
              <th class="num sticky col-rank">RK</th>
              <th class="sticky col-name">Wrestler</th>
              <th>School</th>
              <th>YR</th>
              <th v-if="hasConference">CONF</th>
              <th v-if="hasRecord">RECORD</th>
              <th class="num">Move</th>
            </tr>
          </thead>
          <tbody id="edition-entries">
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
              <td class="num rank sticky col-rank" :class="{ top: row.rank <= 3 }">{{ row.rank }}</td>
              <!-- ".last" prepends the space inside the expression itself
                   (not template whitespace before the mustache — Vue's
                   compiler trims that at an element's edge) so accessible
                   name / text content reads "First Last", not "FirstLast".
                   Harmless for the block-stacked visual layout: whitespace
                   between block elements has no layout effect. -->
              <td class="name sticky col-name">
                <button
                  v-if="row.wrestlerId !== null"
                  type="button"
                  class="row-toggle"
                  :aria-pressed="rowSelectionIndex(row.wrestlerId) >= 0"
                  @click.stop="toggleWrestler(row.wrestlerId)"
                >
                  <span class="first">{{ splitName(row.name).first }}</span>
                  <span v-if="splitName(row.name).last" class="last">{{ ' ' + splitName(row.name).last }}</span>
                </button>
                <span v-else>
                  <span class="first">{{ splitName(row.name).first }}</span>
                  <span v-if="splitName(row.name).last" class="last">{{ ' ' + splitName(row.name).last }}</span>
                </span>
              </td>
              <td class="school">
                <button
                  v-if="row.school"
                  type="button"
                  class="school-toggle"
                  @click.stop="toggleSchool(row.school)"
                >{{ row.school }}</button>
                <span v-else>{{ row.school }}</span>
              </td>
              <td class="grade">{{ row.grade }}</td>
              <td v-if="hasConference" class="grade">{{ row.conference }}</td>
              <td v-if="hasRecord" class="grade">{{ row.record }}</td>
              <td class="num"><MovementBadge :rank="row.rank" :prev-rank="row.prevRank" :prev-weight="row.prevWeight" /></td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="foldedCount > 0" class="fold-toggle">
        <button
          :aria-expanded="expanded"
          aria-controls="edition-entries"
          @click="expanded = !expanded"
        >
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
  <div v-else-if="notYetPublished">
    <div class="pagehead">
      <div>
        <h1>{{ weight }}<small>LBS</small></h1>
        <p class="sub">{{ currentSourceName }} / NOT YET PUBLISHED</p>
        <nav class="source-tabs" aria-label="Ranking source">
          <button
            v-for="s in SOURCES"
            :key="s.slug"
            type="button"
            :class="{ active: s.slug === sourceSlug }"
            :aria-current="s.slug === sourceSlug ? 'true' : undefined"
            @click="switchSource(s.slug)"
          >
            {{ s.name }}
          </button>
        </nav>
        <NuxtLink :to="`/ballot/${weight}`" class="ballot-cta">Build your {{ weight }} ballot →</NuxtLink>
      </div>
    </div>
    <p class="source-empty">
      {{
        sourceSlug === 'fan-poll'
          ? `The Fan Poll hasn't published a ${weight} lbs ranking yet — not enough ballots have been submitted at this weight. Try another source above, or check back once more ballots come in.`
          : `${currentSourceName} hasn't published a ${weight} lbs ranking yet. Try another source above.`
      }}
    </p>
  </div>
</template>

<style scoped>
/* Sticky column offsets for this page's table (RK + Wrestler) — see
   main.css .board td.sticky/.board th.sticky for the shared mechanism.
   .col-rank matches .board td.rank's existing 3rem width so .col-name's
   offset lines up exactly. */
.col-rank {
  left: 0;
  width: 3rem;
}

/* table-layout:auto sizes a column to its widest cell — a sticky cell
   without its own bound drags that FULL natural width along when pinned,
   which visually overlaps the columns after it (the browser doesn't
   re-flow siblings around a sticky element's offset). Bound it; first/last
   name render as two stacked lines (below) instead of one truncated line,
   so only an unusually long single name part ever needs the ellipsis
   fallback. */
.col-name {
  left: 3rem;
  width: 7rem;
  max-width: 7rem;
  box-shadow: 2px 0 4px -2px rgb(0 0 0 / 25%);
}

.col-name .first,
.col-name .last {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.col-name .last {
  color: var(--muted);
  font-weight: 500;
}
</style>
