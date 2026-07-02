<script setup lang="ts">
import { movement } from '~/utils/movement'

const props = defineProps<{
  rank: number
  prevRank: number | null
}>()

const move = computed(() => movement(props.rank, props.prevRank))
</script>

<template>
  <span class="mv" :class="move.kind">
    <template v-if="move.kind === 'new'">NEW</template>
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
</style>
