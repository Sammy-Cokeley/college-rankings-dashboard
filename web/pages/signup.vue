<script setup lang="ts">
const { fetch: refreshSession } = useUserSession()
const route = useRoute()
// e.g. the ballot builder links here as /signup?redirect=/ballot/149 so
// signing up returns the user to what they were doing, not the home page.
const redirect = computed(() => {
  const r = route.query.redirect
  return typeof r === 'string' && r.startsWith('/') ? r : '/'
})

const email = ref('')
const password = ref('')
const displayName = ref('')
const submitting = ref(false)
const errorMessage = ref('')

async function onSubmit() {
  errorMessage.value = ''
  submitting.value = true
  try {
    await $fetch('/api/auth/signup', {
      method: 'POST',
      body: {
        email: email.value,
        password: password.value,
        displayName: displayName.value.trim() || undefined,
      },
    })
    await refreshSession()
    await navigateTo(redirect.value)
  } catch (err: unknown) {
    errorMessage.value =
      (err as { data?: { statusMessage?: string } })?.data?.statusMessage ?? 'Sign up failed'
  } finally {
    submitting.value = false
  }
}

useSeoMeta({ title: 'Sign up — NCAA DI Wrestling Rankings' })
</script>

<template>
  <div class="auth-page">
    <h1>Sign up</h1>
    <form class="auth-form" @submit.prevent="onSubmit">
      <label>
        Email
        <input v-model="email" type="email" autocomplete="email" required>
      </label>
      <label>
        Password
        <input
          v-model="password"
          type="password"
          autocomplete="new-password"
          minlength="8"
          required
        >
      </label>
      <label>
        Display name <span class="optional">(optional)</span>
        <input v-model="displayName" type="text" autocomplete="nickname">
      </label>
      <p v-if="errorMessage" class="error" role="alert">{{ errorMessage }}</p>
      <button type="submit" :disabled="submitting">
        {{ submitting ? 'Creating account…' : 'Sign up' }}
      </button>
    </form>
    <p class="switch">
      Already have an account?
      <NuxtLink :to="{ path: '/login', query: route.query }">Log in</NuxtLink>
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

.optional {
  font-weight: normal;
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
