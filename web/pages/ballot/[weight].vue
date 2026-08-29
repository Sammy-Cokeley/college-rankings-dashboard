<script setup lang="ts">
import type { BallotEntry, WrestlerOption } from '~/types/ballots'
import type { RankingRow, WeightRankings } from '~/types/rankings'
import { isWeightClass } from '~/utils/weights'
import { SOURCES } from '~/utils/sources'

definePageMeta({
  validate: (route) => isWeightClass(Number(route.params.weight)),
})

const route = useRoute()
// Reactive, not a plain const: the router reuses this component instance
// when navigating between ballot weight pages ([weight].vue's own comment
// notes the same gotcha).
const weight = computed(() => Number(route.params.weight))

const { loggedIn } = useUserSession()

// --- reference panel: an existing ranking to check yourself against while
// building, instead of needing another tab/site (feedback from the first
// real user test session) ------------------------------------------------

const referenceSourceSlug = ref(SOURCES[0]!.slug) // FloWrestling by default
const referencePanelOpen = ref(true)
const referenceUrl = computed(
  () => `/api/rankings/${weight.value}?source=${referenceSourceSlug.value}`,
)
// No error re-throw here (unlike the main rankings pages): missing reference
// data — e.g. the Fan Poll before a weight has 3 ballots — should degrade to
// an empty-state message, never break the ballot builder itself.
const { data: referenceData } = await useFetch<WeightRankings>(referenceUrl)
const referenceEntries = computed<RankingRow[]>(() => referenceData.value?.edition.entries ?? [])

const entries = ref<BallotEntry[]>([])
const loaded = ref(false)
type SaveState = 'idle' | 'saving' | 'saved' | 'error'
const saveState = ref<SaveState>('idle')

function storageKey(w: number) {
  return `ballot-draft-${w}`
}

async function loadEntries() {
  loaded.value = false
  saveState.value = 'idle'
  if (loggedIn.value) {
    const ballot = await $fetch<{ entries: BallotEntry[] }>(`/api/ballots/${weight.value}`)
    entries.value = ballot.entries
  } else if (import.meta.client) {
    const raw = localStorage.getItem(storageKey(weight.value))
    entries.value = raw ? (JSON.parse(raw) as BallotEntry[]) : []
  }
  // The `entries` watch below fires asynchronously (Vue's default flush
  // timing), not synchronously on assignment — setting `loaded.value = true`
  // right here would win the race and run BEFORE the watcher's own callback
  // sees it, defeating the guard entirely (confirmed via manual testing: it
  // fired a spurious save on every page load). Waiting a tick first lets that
  // pending watcher callback run — and bail out — while loaded is still false.
  await nextTick()
  loaded.value = true
}

onMounted(loadEntries)
watch(weight, loadEntries)

let debounceTimer: ReturnType<typeof setTimeout> | undefined
watch(
  entries,
  () => {
    // Skip the save the initial load() itself triggers (entries.value = ...
    // fires this watcher too) — only real edits after loading should autosave.
    if (!loaded.value) return
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(persist, 500)
  },
  { deep: true },
)

async function persist() {
  if (loggedIn.value) {
    saveState.value = 'saving'
    try {
      await $fetch(`/api/ballots/${weight.value}`, {
        method: 'PATCH',
        body: { wrestlerIds: entries.value.map((e) => e.wrestlerId) },
      })
      saveState.value = 'saved'
    } catch {
      saveState.value = 'error'
    }
  } else {
    // Guests: client-side only, never sent to the write API — the API is
    // auth-guarded anyway, so there's no server round trip to gate here.
    localStorage.setItem(storageKey(weight.value), JSON.stringify(entries.value))
  }
}

function renumber(list: BallotEntry[]): BallotEntry[] {
  return list.map((e, i) => ({ ...e, rank: i + 1 }))
}

function addWrestler(w: WrestlerOption) {
  if (entries.value.length >= 33) return
  if (entries.value.some((e) => e.wrestlerId === w.wrestlerId)) return
  entries.value = renumber([
    ...entries.value,
    { rank: 0, wrestlerId: w.wrestlerId, name: w.name, school: w.school },
  ])
}

function removeWrestler(wrestlerId: number) {
  entries.value = renumber(entries.value.filter((e) => e.wrestlerId !== wrestlerId))
}

function moveUp(index: number) {
  if (index <= 0) return
  const copy = [...entries.value]
  ;[copy[index - 1], copy[index]] = [copy[index]!, copy[index - 1]!]
  entries.value = renumber(copy)
}

function moveDown(index: number) {
  if (index >= entries.value.length - 1) return
  const copy = [...entries.value]
  ;[copy[index], copy[index + 1]] = [copy[index + 1]!, copy[index]!]
  entries.value = renumber(copy)
}

const excludeIds = computed(() => entries.value.map((e) => e.wrestlerId))
const redirectQuery = computed(() => ({ redirect: route.fullPath }))

useSeoMeta({ title: () => `Build your ${weight.value} ballot — NCAA DI Wrestling Rankings` })
</script>

<template>
  <div class="ballot-page">
    <div class="pagehead">
      <div>
        <h1>{{ weight }} Ballot</h1>
        <p class="sub">{{ entries.length ? `YOUR TOP ${entries.length}` : 'BUILD YOUR RANKING' }} · FAN POLL</p>
      </div>
      <div class="status">
        <span v-if="!loggedIn" class="guest-note">
          <NuxtLink :to="{ path: '/signup', query: redirectQuery }">Sign up</NuxtLink>
          or
          <NuxtLink :to="{ path: '/login', query: redirectQuery }">log in</NuxtLink>
          to save your ballot and have it count.
        </span>
        <span v-else-if="saveState === 'saving'" class="save-indicator">Saving…</span>
        <span v-else-if="saveState === 'saved'" class="save-indicator saved">Saved</span>
        <span v-else-if="saveState === 'error'" class="save-indicator error">
          Couldn't save — check your connection
        </span>
      </div>
    </div>

    <NuxtLink :to="`/${weight}`" class="back-link">← Back to {{ weight }} rankings</NuxtLink>

    <div class="builder-layout">
      <div class="builder">
        <WrestlerSearch :weight="weight" :exclude-ids="excludeIds" @select="addWrestler" />

        <ol v-if="entries.length > 0" class="ballot-list">
          <li v-for="(entry, index) in entries" :key="entry.wrestlerId">
            <span class="rank">{{ entry.rank }}</span>
            <span class="name">{{ entry.name }}</span>
            <span class="school">{{ entry.school }}</span>
            <span class="reorder">
              <button type="button" :disabled="index === 0" aria-label="Move up" @click="moveUp(index)">
                ↑
              </button>
              <button
                type="button"
                :disabled="index === entries.length - 1"
                aria-label="Move down"
                @click="moveDown(index)"
              >
                ↓
              </button>
              <button type="button" aria-label="Remove" class="remove" @click="removeWrestler(entry.wrestlerId)">
                ✕
              </button>
            </span>
          </li>
        </ol>
        <p v-else-if="loaded" class="empty">
          Search above to add wrestlers. A top 10 is a solid ballot — keep going up to 33 if
          you want to go deeper.
        </p>
      </div>

      <aside class="reference-panel" :class="{ collapsed: !referencePanelOpen }">
        <div class="reference-head">
          <button type="button" class="panel-toggle" @click="referencePanelOpen = !referencePanelOpen">
            <span class="chevron">{{ referencePanelOpen ? '▾' : '▸' }}</span> Reference
          </button>
          <nav v-if="referencePanelOpen" class="source-tabs" aria-label="Reference source">
            <button
              v-for="s in SOURCES"
              :key="s.slug"
              type="button"
              :class="{ active: s.slug === referenceSourceSlug }"
              @click="referenceSourceSlug = s.slug"
            >
              {{ s.name }}
            </button>
          </nav>
        </div>
        <ol v-if="referencePanelOpen && referenceEntries.length > 0" class="reference-list">
          <li v-for="row in referenceEntries" :key="`${row.rank}-${row.name}`">
            <span class="rank">{{ row.rank }}</span>
            <span class="name">{{ row.name }}</span>
            <span class="school">{{ row.school }}</span>
          </li>
        </ol>
        <p v-else-if="referencePanelOpen" class="empty">
          {{
            referenceSourceSlug === 'fan-poll'
              ? 'Not enough ballots yet for a Fan Poll ranking at this weight.'
              : 'No rankings available for this weight yet.'
          }}
        </p>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.ballot-page {
  padding-bottom: 2rem;
}

.status {
  font-size: 0.85rem;
  color: var(--muted);
}

.guest-note a {
  color: var(--accent);
}

.save-indicator.saved {
  color: var(--up);
}

.save-indicator.error {
  color: var(--down);
}

.builder-layout {
  display: grid;
  grid-template-columns: 1fr;
  gap: 1.5rem;
  align-items: start;
}

@media (min-width: 56rem) {
  .builder-layout {
    grid-template-columns: 1fr 20rem;
  }
}

.reference-panel {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 0.75rem;
  position: sticky;
  top: 1rem;
}

.reference-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.panel-toggle {
  background: none;
  border: none;
  color: var(--ink);
  font-weight: 600;
  cursor: pointer;
  padding: 0;
}

.chevron {
  color: var(--accent);
  display: inline-block;
  width: 1rem;
}

.source-tabs {
  display: flex;
  gap: 0.35rem;
}

.source-tabs button {
  background: none;
  border: 1px solid var(--line);
  border-radius: 4px;
  color: var(--muted);
  font-size: 0.8rem;
  padding: 0.15rem 0.5rem;
  cursor: pointer;
}

.source-tabs button.active {
  color: var(--ink);
  border-color: var(--accent);
}

.reference-list {
  list-style: none;
  margin: 0.75rem 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  max-height: 32rem;
  overflow-y: auto;
}

.reference-list li {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.85rem;
  padding: 0.25rem 0;
  border-bottom: 1px solid var(--line);
}

.reference-list .rank {
  font-family: var(--font-mono);
  color: var(--accent);
  width: 1.5rem;
  text-align: right;
  flex-shrink: 0;
}

.reference-list .name {
  flex: 1;
}

.reference-list .school {
  color: var(--muted);
  font-size: 0.8rem;
}

.reference-panel .empty {
  margin-top: 0.75rem;
  font-size: 0.85rem;
}

.ballot-list {
  list-style: none;
  margin: 1.25rem 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.ballot-list li {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 0.5rem 0.75rem;
}

.rank {
  font-family: var(--font-mono);
  color: var(--accent);
  width: 2rem;
  text-align: right;
}

.name {
  flex: 1;
}

.school {
  color: var(--muted);
  font-size: 0.9rem;
}

.reorder {
  display: flex;
  gap: 0.25rem;
}

.reorder button {
  background: none;
  border: 1px solid var(--line);
  border-radius: 4px;
  color: var(--ink);
  width: 1.75rem;
  height: 1.75rem;
  cursor: pointer;
}

.reorder button:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.reorder button.remove {
  color: var(--down);
}

.empty {
  color: var(--muted);
  margin-top: 1.5rem;
}
</style>
