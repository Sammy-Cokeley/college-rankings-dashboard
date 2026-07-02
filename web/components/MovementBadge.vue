<script setup lang="ts">
import { movement } from '~/utils/movement'

const props = defineProps<{
  rank: number
  prevRank: number | null
}>()

const move = computed(() => movement(props.rank, props.prevRank))
</script>

<template>
  <span class="movement" :class="move.kind">
    <template v-if="move.kind === 'new'">NEW</template>
    <template v-else-if="move.kind === 'up'">▲{{ move.delta }}</template>
    <template v-else-if="move.kind === 'down'">▼{{ move.delta }}</template>
    <template v-else>—</template>
  </span>
</template>

<style scoped>
.movement {
  font-size: 0.85rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.movement.up {
  color: var(--up);
}

.movement.down {
  color: var(--down);
}

.movement.even {
  color: var(--muted);
}

.movement.new {
  color: var(--accent);
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.05em;
}
</style>
