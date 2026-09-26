<script setup lang="ts">
import type { BallotSubmission } from '~/types/ballots'

const { loggedIn, user } = useUserSession()

// The first HARD-gated page in the app — every other gate (the ballot
// builder) is soft/optional, since building a ballot anonymously is a real,
// supported flow. A profile has nothing to show a logged-out visitor, so
// this redirects outright rather than rendering an empty shell. Same
// ?redirect= convention the ballot builder's login/signup links already
// use — no new middleware infrastructure needed for one page.
if (!loggedIn.value) {
  await navigateTo('/login?redirect=/profile')
}

const { data: submissions } = await useFetch<BallotSubmission[]>('/api/profile/ballots', {
  // Skip the fetch entirely while the redirect above is landing — avoids a
  // pointless 401 round-trip for a logged-out visitor mid-redirect.
  immediate: loggedIn.value,
})

useSeoMeta({ title: 'Your profile — NCAA DI Wrestling Rankings' })
</script>

<template>
  <div class="profile-page">
    <div class="pagehead">
      <div>
        <h1>Your Ballots</h1>
        <p class="sub">{{ user?.displayName || user?.email }}</p>
      </div>
    </div>

    <section class="board history">
      <h2>Previous submissions</h2>
      <ol v-if="submissions && submissions.length > 0" class="history-list">
        <li v-for="s in submissions" :key="s.id">
          <div class="history-head">
            <NuxtLink :to="`/ballot/${s.weightClass}`" class="weight">{{ s.weightClass }} lbs</NuxtLink>
            <span class="submitted-at">{{ new Date(s.submittedAt).toLocaleString() }}</span>
          </div>
          <ol class="picks">
            <li v-for="e in s.entries" :key="e.wrestlerId">
              <span class="rank">{{ e.rank }}</span>
              <span class="name">{{ e.name }}</span>
              <span class="school">{{ e.school }}</span>
            </li>
          </ol>
        </li>
      </ol>
      <p v-else-if="submissions" class="empty">
        No submissions yet — build a ballot and hit Submit to start your history.
      </p>
    </section>
  </div>
</template>

<style scoped>
.profile-page {
  padding-bottom: 2rem;
}

.history {
  margin-top: 1.5rem;
  padding: 1.25rem 1.5rem 1.5rem;
}

.history h2 {
  margin: 0 0 1rem;
  font-size: 0.8rem;
  font-family: var(--font-mono);
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--muted);
}

.history-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.history-list > li {
  border: 1px solid var(--line);
  border-radius: 0.5rem;
  padding: 0.9rem 1rem;
}

.history-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.6rem;
}

.history-head .weight {
  font-weight: 700;
  color: var(--accent);
  text-decoration: none;
}

.history-head .weight:hover {
  text-decoration: underline;
}

.submitted-at {
  font-family: var(--font-mono);
  font-size: 0.78rem;
  color: var(--muted);
}

.picks {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.picks li {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.3rem 0.1rem;
  font-size: 0.88rem;
}

.picks .rank {
  font-family: var(--font-mono);
  font-weight: 600;
  color: var(--muted);
  width: 1.5rem;
  text-align: right;
}

.picks .name {
  font-weight: 600;
  flex: 1;
}

.picks .school {
  color: var(--muted);
  font-size: 0.85rem;
}

.empty {
  color: var(--muted);
  margin: 0;
}
</style>
