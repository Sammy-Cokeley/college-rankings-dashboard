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
          <NuxtLink to="/rankings">All Weights</NuxtLink>
          <NuxtLink v-for="w in WEIGHT_CLASSES" :key="w" :to="`/${w}`">{{ w }}</NuxtLink>
        </nav>
        <button class="theme-toggle" aria-label="Toggle color theme" @click="toggle">
          ◐ theme
        </button>
        <AuthState>
          <template #default="{ loggedIn, user, clear }">
            <span v-if="loggedIn" class="auth-status">
              {{ user?.displayName || user?.email }}
              <button class="auth-link" @click="clear">Log out</button>
            </span>
            <NuxtLink v-else to="/login" class="auth-link">Log in</NuxtLink>
          </template>
          <template #placeholder>
            <span class="auth-status" />
          </template>
        </AuthState>
      </div>
    </header>
    <main class="site-main wrap">
      <NuxtPage />
    </main>
    <footer class="site-footer wrap">
      Rankings attributed to their source · movement derived week-over-week
    </footer>
  </div>
</template>
