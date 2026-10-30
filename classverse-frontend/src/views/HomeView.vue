<script setup lang="ts">
import AppShell from '@/components/AppShell.vue'
import { useOrgStore } from '@/stores/org'

const org = useOrgStore()
</script>

<template>
  <AppShell>
    <section class="placeholder" aria-labelledby="map-title">
      <h1 id="map-title" class="title">{{ org.active?.name }}</h1>
      <p class="sub" data-testid="home-info">
        Peta sekolah akan muncul di sini. Kamu masuk sebagai
        {{
          { admin: 'admin', teacher: 'pengajar', participant: 'peserta' }[
            org.active?.role ?? 'participant'
          ]
        }}.
      </p>
    </section>
  </AppShell>
</template>

<style scoped>
.placeholder {
  padding: var(--space-10) var(--space-8);
  border-radius: var(--radius-2xl);
  background: var(--map-grass);
}

.title {
  margin: 0;
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  font-size: var(--text-3xl);
}

.sub {
  margin: var(--space-3) 0 0;
  font-size: var(--text-lg);
}
</style>
