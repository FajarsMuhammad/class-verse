<script setup lang="ts">
import logoUrl from '@/assets/avatars/round.svg'

defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <main class="auth">
    <section class="card" :aria-labelledby="'auth-title'">
      <header class="brand">
        <img :src="logoUrl" alt="" width="40" height="40" />
        <span class="wordmark">Kelasverse</span>
      </header>
      <h1 id="auth-title" class="title">{{ title }}</h1>
      <p v-if="subtitle" class="subtitle">{{ subtitle }}</p>
      <slot />
    </section>
  </main>
</template>

<style scoped>
.auth {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: var(--space-6);
  box-sizing: border-box;
}

.card {
  width: 100%;
  max-width: 440px;
  box-sizing: border-box;
  padding: var(--space-8);
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
}

.brand {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin-bottom: var(--space-6);
}

.brand img {
  width: var(--space-10);
  height: var(--space-10);
}

.wordmark {
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  font-size: var(--text-xl);
}

.title {
  margin: 0;
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  font-size: var(--text-2xl);
  line-height: var(--line-tight);
}

.subtitle {
  margin: var(--space-2) 0 0;
  color: var(--color-ink-muted);
  font-size: var(--text-md);
}
</style>
