<script setup lang="ts">
const { fetch: refreshSession } = useUserSession()
const route = useRoute()
// See signup.vue for why: e.g. the ballot builder links here with
// ?redirect=/ballot/149 so logging in returns to what the user was doing.
const redirect = computed(() => {
  const r = route.query.redirect
  return typeof r === 'string' && r.startsWith('/') ? r : '/'
})

const email = ref('')
const password = ref('')
const submitting = ref(false)
const errorMessage = ref('')

async function onSubmit() {
  errorMessage.value = ''
  submitting.value = true
  try {
    await $fetch('/api/auth/login', {
      method: 'POST',
      body: { email: email.value, password: password.value },
    })
    await refreshSession()
    await navigateTo(redirect.value)
  } catch (err: unknown) {
    errorMessage.value =
      (err as { data?: { statusMessage?: string } })?.data?.statusMessage ?? 'Log in failed'
  } finally {
    submitting.value = false
  }
}

useSeoMeta({ title: 'Log in — NCAA DI Wrestling Rankings' })
</script>

<template>
  <div class="auth-page">
    <h1>Log in</h1>
    <form class="auth-form" @submit.prevent="onSubmit">
      <label>
        Email
        <input v-model="email" type="email" autocomplete="email" required>
      </label>
      <label>
        Password
        <input v-model="password" type="password" autocomplete="current-password" required>
      </label>
      <p v-if="errorMessage" class="error" role="alert">{{ errorMessage }}</p>
      <button type="submit" :disabled="submitting">
        {{ submitting ? 'Logging in…' : 'Log in' }}
      </button>
    </form>
    <p class="switch">
      Don't have an account?
      <NuxtLink :to="{ path: '/signup', query: route.query }">Sign up</NuxtLink>
    </p>
  </div>
</template>

<style scoped>
.auth-page {
  max-width: 24rem;
  margin: 3rem auto;
}

.auth-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  margin-top: 1.5rem;
}

label {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-size: 0.85rem;
  color: var(--muted);
}

input {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 6px;
  color: var(--ink);
  padding: 0.6rem 0.75rem;
  font: inherit;
}

button {
  background: var(--accent);
  color: var(--accent-ink);
  border: none;
  border-radius: 6px;
  padding: 0.7rem 1rem;
  font-weight: 600;
  cursor: pointer;
}

button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.error {
  color: var(--down);
  font-size: 0.9rem;
  margin: 0;
}

.switch {
  margin-top: 1.5rem;
  font-size: 0.9rem;
  color: var(--muted);
}
</style>
