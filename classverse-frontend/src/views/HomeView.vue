<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { checkHealth, type BackendStatus } from '@/api/health'
import avatarUrl from '@/assets/avatars/round.svg'

const status = ref<BackendStatus | 'checking'>('checking')

const label = computed(() => {
  switch (status.value) {
    case 'ok':
      return 'Server terhubung'
    case 'down':
      return 'Server tidak terhubung'
    default:
      return 'Memeriksa server…'
  }
})

async function refresh() {
  status.value = 'checking'
  status.value = await checkHealth()
}

onMounted(refresh)
</script>

<template>
  <main class="home">
    <header class="brand">
      <img class="avatar" :src="avatarUrl" alt="" width="64" height="64" />
      <h1 class="wordmark">Kelasverse</h1>
    </header>
    <p class="tagline">Ruang belajar 2D yang hangat, untuk SMA, kampus, dan nanti semua usia.</p>

    <section class="card" aria-labelledby="status-title">
      <h2 id="status-title" class="card-title">Status backend</h2>
      <p class="badge" :class="`badge--${status}`" role="status" data-testid="backend-status">
        {{ label }}
      </p>
      <button
        type="button"
        class="btn-secondary"
        :disabled="status === 'checking'"
        @click="refresh"
      >
        Periksa lagi
      </button>
    </section>
  </main>
</template>

<style scoped>
.home {
  max-width: 720px;
  margin: 0 auto;
  padding: var(--space-14) var(--space-6);
}

.brand {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

.avatar {
  width: var(--space-14);
  height: var(--space-14);
}

.wordmark {
  margin: 0;
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  font-size: var(--text-4xl);
  line-height: var(--line-tight);
}

.tagline {
  margin: var(--space-3) 0 var(--space-8);
  font-size: var(--text-lg);
  color: var(--color-ink-muted);
}

.card {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-4);
  padding: var(--space-6) var(--space-8);
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
}

.card-title {
  flex-basis: 100%;
  margin: 0;
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  font-size: var(--text-2xl);
}

.badge {
  margin: 0;
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-pill);
  font-size: var(--text-md);
  font-weight: var(--weight-bold);
}

.badge--checking {
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
}

.badge--ok {
  background: var(--color-live-soft);
  color: var(--color-live-ink);
}

.badge--down {
  background: var(--color-accent-soft);
  color: var(--color-accent-ink);
}

.btn-secondary {
  min-height: var(--touch-min);
  padding: 0 var(--space-5);
  border: 2px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
  color: var(--color-ink);
  font: inherit;
  font-weight: var(--weight-bold);
  cursor: pointer;
}

.btn-secondary:disabled {
  cursor: progress;
  color: var(--color-ink-muted);
}
</style>
