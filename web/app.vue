<script setup lang="ts">
import { WEIGHT_CLASSES } from './utils/weights'

const { toggle } = useTheme()
</script>

<template>
  <div>
    <header class="topbar">
      <div class="inner">
        <NuxtLink to="/" class="brand">NCAA DI Wrestling <b>Rankings</b></NuxtLink>
        <nav class="weight-nav" aria-label="Weight classes">
          <NuxtLink v-for="w in WEIGHT_CLASSES" :key="w" :to="`/${w}`">{{ w }}</NuxtLink>
        </nav>
        <button class="theme-toggle" aria-label="Toggle color theme" @click="toggle">
          ◐ theme
        </button>
      </div>
    </header>
    <main class="site-main wrap">
      <NuxtPage />
    </main>
    <footer class="site-footer wrap">
      Rankings as published by FloWrestling · movement derived week-over-week
    </footer>
  </div>
</template>
