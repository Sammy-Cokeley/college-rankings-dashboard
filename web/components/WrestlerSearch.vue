<script setup lang="ts">
import type { WrestlerOption } from '~/types/ballots'

const props = defineProps<{
  weight: number
  excludeIds: number[] // already on the ballot; filtered out of results
}>()

const emit = defineEmits<{ select: [wrestler: WrestlerOption] }>()

const query = ref('')
const results = ref<WrestlerOption[]>([])
const loading = ref(false)
let debounceTimer: ReturnType<typeof setTimeout> | undefined

async function runSearch() {
  loading.value = true
  try {
    const data = await $fetch<WrestlerOption[]>('/api/wrestlers/search', {
      query: { weight: props.weight, q: query.value },
    })
    results.value = data
  } finally {
    loading.value = false
  }
}

watch(
  () => [query.value, props.weight] as const,
  () => {
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(runSearch, 300)
  },
  { immediate: true },
)

const visibleResults = computed(() =>
  results.value.filter((w) => !props.excludeIds.includes(w.wrestlerId)),
)

function select(wrestler: WrestlerOption) {
  emit('select', wrestler)
  query.value = ''
}
</script>

<template>
  <div class="wrestler-search">
    <input
      v-model="query"
      type="search"
      placeholder="Search any D1 wrestler…"
      aria-label="Search wrestlers to add to your ballot"
    >
    <ul v-if="visibleResults.length > 0" class="results">
      <li v-for="w in visibleResults" :key="w.wrestlerId">
        <button type="button" @click="select(w)">
          <span class="name">{{ w.name }}</span>
          <span class="meta">
            {{ w.school }}
            <span v-if="w.weightClass !== weight" class="off-weight">
              · listed at {{ w.weightClass ?? '?' }}
            </span>
          </span>
        </button>
      </li>
    </ul>
    <p v-else-if="loading" class="hint">Searching…</p>
    <p v-else-if="query" class="hint">No matches.</p>
  </div>
</template>

<style scoped>
.wrestler-search {
  position: relative;
}

input {
  width: 100%;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 6px;
  color: var(--ink);
  padding: 0.6rem 0.75rem;
  font: inherit;
}

.results {
  list-style: none;
  margin: 0.4rem 0 0;
  padding: 0;
  max-height: 16rem;
  overflow-y: auto;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
}

.results button {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 0.75rem;
  width: 100%;
  padding: 0.5rem 0.75rem;
  background: none;
  border: none;
  color: var(--ink);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.results button:hover {
  background: var(--surface-hover);
}

.meta {
  color: var(--muted);
  font-size: 0.85rem;
  white-space: nowrap;
}

.off-weight {
  color: var(--accent);
}

.hint {
  color: var(--muted);
  font-size: 0.85rem;
  margin: 0.4rem 0 0;
}
</style>
