<script setup lang="ts">
import { movement } from '~/utils/movement'

const props = defineProps<{
  rank: number
  prevRank: number | null
  prevWeight?: { weight: number; rank: number } | null
}>()

const move = computed(() => movement(props.rank, props.prevRank))
</script>

<template>
  <span class="mv" :class="[move.kind, { annotated: move.kind === 'new' && prevWeight }]">
    <template v-if="move.kind === 'new'">
      <template v-if="prevWeight">NEW — previously #{{ prevWeight.rank }} at {{ prevWeight.weight }}</template>
      <template v-else>NEW</template>
    </template>
    <template v-else-if="move.kind === 'up'">▲{{ move.delta }}</template>
    <template v-else-if="move.kind === 'down'">▼{{ move.delta }}</template>
    <template v-else>·</template>
  </span>
</template>

<style scoped>
.mv {
  font-family: var(--font-mono);
  font-size: 0.85rem;
  font-weight: 600;
  white-space: nowrap;
}

.mv.up {
  color: var(--up);
}

.mv.down {
  color: var(--down);
}

.mv.even {
  color: var(--muted);
  font-weight: 400;
}

.mv.new {
  color: var(--accent);
  font-size: 0.72rem;
  letter-spacing: 0.1em;
}

/* The cross-weight annotation is a full clause, not a 3-letter badge — the
   nowrap/letter-spacing tuned for "NEW" makes a long sentence unreadable and
   blows out the table's Move column width. */
.mv.annotated {
  display: inline-block;
  white-space: normal;
  letter-spacing: normal;
  max-width: 11rem;
  text-align: right;
}
</style>
