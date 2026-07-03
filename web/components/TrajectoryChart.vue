<script setup lang="ts">
import type { EditionDate, WrestlerSeries } from '~/types/rankings'
import { chartScales, polylinePoints, seriesSegments, type ChartBox } from '~/utils/chart'

const props = defineProps<{
  weeks: EditionDate[]
  series: WrestlerSeries[]
  selected: number[]
}>()

const emit = defineEmits<{ toggle: [wrestlerId: number] }>()

const hovered = ref<number | null>(null)

const box: ChartBox = { width: 860, height: 440, padTop: 16, padRight: 132, padBottom: 32, padLeft: 36 }

const maxRank = computed(() =>
  Math.max(1, ...props.series.flatMap((s) => s.points.map((p) => p.rank))),
)
const scales = computed(() => chartScales(props.weeks.length, maxRank.value, box))

const rankTicks = computed(() => {
  const ticks = [1]
  for (let r = 5; r <= maxRank.value; r += 5) ticks.push(r)
  return ticks
})
const weekTicks = computed(() => props.weeks.filter((w) => w.week === 1 || w.week % 3 === 0))

function isEmphasized(s: WrestlerSeries): boolean {
  return props.selected.includes(s.wrestlerId) || hovered.value === s.wrestlerId
}

// SVG has no z-index: paint order is document order, so emphasized lines are
// rendered last to sit on top of the muted spaghetti.
const paintOrder = computed(() => {
  const muted = props.series.filter((s) => !isEmphasized(s))
  return [...muted, ...props.series.filter(isEmphasized)]
})

function stateClass(s: WrestlerSeries): string {
  const i = props.selected.indexOf(s.wrestlerId)
  if (i >= 0) return `sel sel-${i % 6}`
  return hovered.value === s.wrestlerId ? 'hovered' : ''
}

function segments(s: WrestlerSeries) {
  return seriesSegments(s.points)
}

function endLabel(s: WrestlerSeries) {
  const last = s.points[s.points.length - 1]!
  return { x: scales.value.x(last.week) + 7, y: scales.value.y(last.rank) + 3.5 }
}

const poly = (seg: { week: number; rank: number }[]) => polylinePoints(seg, scales.value)
</script>

<template>
  <figure class="chart">
    <!-- NOTE: dynamic SVG geometry uses the .attr binding modifier. During
         hydration Vue force-patches dynamic props without the svg namespace
         and hits read-only DOM interfaces (SVGAnimatedLength etc.), spamming
         warnings; .attr forces the setAttribute path. -->
    <svg :viewBox.attr="`0 0 ${box.width} ${box.height}`" role="img"
         aria-label="Rank trajectory of every ranked wrestler across the season">
      <!-- grid + axes -->
      <g class="grid">
        <template v-for="r in rankTicks" :key="`r${r}`">
          <line :x1.attr="box.padLeft" :x2.attr="box.width - box.padRight"
                :y1.attr="scales.y(r)" :y2.attr="scales.y(r)" />
          <text :x.attr="box.padLeft - 8" :y.attr="scales.y(r) + 3.5" text-anchor="end">{{ r }}</text>
        </template>
        <template v-for="w in weekTicks" :key="`w${w.week}`">
          <text :x.attr="scales.x(w.week)" :y.attr="box.height - 10" text-anchor="middle">WK{{ w.week }}</text>
        </template>
      </g>

      <!-- series lines (muted first, emphasized painted on top) -->
      <g v-for="s in paintOrder" :key="s.wrestlerId" class="series" :class="stateClass(s)">
        <template v-for="(seg, i) in segments(s)" :key="i">
          <polyline v-if="seg.length > 1" class="line" :points.attr="poly(seg)" />
          <circle v-else class="dot lone" :cx.attr="scales.x(seg[0]!.week)" :cy.attr="scales.y(seg[0]!.rank)" r="2.5" />
        </template>
        <template v-if="isEmphasized(s)">
          <circle v-for="p in s.points" :key="`d${p.week}`" class="dot"
                  :cx.attr="scales.x(p.week)" :cy.attr="scales.y(p.rank)" r="3" />
          <text class="label" :x.attr="endLabel(s).x" :y.attr="endLabel(s).y">{{ s.name }}</text>
        </template>
        <!-- invisible fat hit lines for hover/click -->
        <polyline v-for="(seg, i) in segments(s)" :key="`h${i}`" class="hit"
                  :points.attr="seg.length > 1 ? poly(seg) : `${poly(seg)} ${poly(seg)}`"
                  @mouseenter="hovered = s.wrestlerId"
                  @mouseleave="hovered = null"
                  @click="emit('toggle', s.wrestlerId)">
          <title>{{ s.name }}{{ s.school ? ` — ${s.school}` : '' }}</title>
        </polyline>
      </g>
    </svg>
  </figure>
</template>

<style scoped>
.chart {
  margin: 0;
}

.chart svg {
  display: block;
  width: 100%;
  height: auto;
}

.grid line {
  stroke: var(--line);
  stroke-width: 1;
}

.grid text,
.label {
  font-family: var(--font-mono);
  font-size: 10.5px;
  fill: var(--muted);
}

.line {
  fill: none;
  stroke: var(--muted);
  stroke-opacity: 0.3;
  stroke-width: 1.5;
  stroke-linejoin: round;
  stroke-linecap: round;
}

.dot {
  fill: var(--muted);
  fill-opacity: 0.3;
}

.series.hovered .line,
.series.sel .line {
  stroke-opacity: 1;
  stroke-width: 2.5;
}

.series.hovered .dot,
.series.sel .dot {
  fill-opacity: 1;
}

.series.hovered .line { stroke: var(--ink); }
.series.hovered .dot { fill: var(--ink); }
.series.hovered .label { fill: var(--ink); font-weight: 600; }

.series.sel-0 .line { stroke: var(--chart-0); } .series.sel-0 .dot { fill: var(--chart-0); } .series.sel-0 .label { fill: var(--chart-0); font-weight: 600; }
.series.sel-1 .line { stroke: var(--chart-1); } .series.sel-1 .dot { fill: var(--chart-1); } .series.sel-1 .label { fill: var(--chart-1); font-weight: 600; }
.series.sel-2 .line { stroke: var(--chart-2); } .series.sel-2 .dot { fill: var(--chart-2); } .series.sel-2 .label { fill: var(--chart-2); font-weight: 600; }
.series.sel-3 .line { stroke: var(--chart-3); } .series.sel-3 .dot { fill: var(--chart-3); } .series.sel-3 .label { fill: var(--chart-3); font-weight: 600; }
.series.sel-4 .line { stroke: var(--chart-4); } .series.sel-4 .dot { fill: var(--chart-4); } .series.sel-4 .label { fill: var(--chart-4); font-weight: 600; }
.series.sel-5 .line { stroke: var(--chart-5); } .series.sel-5 .dot { fill: var(--chart-5); } .series.sel-5 .label { fill: var(--chart-5); font-weight: 600; }

.hit {
  fill: none;
  stroke: transparent;
  stroke-width: 12;
  stroke-linecap: round;
  cursor: pointer;
  pointer-events: stroke;
}
</style>
