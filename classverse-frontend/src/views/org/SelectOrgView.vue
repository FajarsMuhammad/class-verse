<script setup lang="ts">
import { RouterLink, useRouter } from 'vue-router'

import AuthCard from '@/components/AuthCard.vue'
import { useOrgStore } from '@/stores/org'

const org = useOrgStore()
const router = useRouter()

const roleLabel = { admin: 'Admin', teacher: 'Pengajar', participant: 'Peserta' } as const

async function choose(id: string) {
  org.select(id)
  await router.push({ name: 'home' })
}
</script>

<template>
  <AuthCard title="Pilih sekolah">
    <ul class="list">
      <li v-for="o in org.orgs" :key="o.id">
        <button
          type="button"
          class="choice"
          :aria-current="o.id === org.active?.id ? 'true' : undefined"
          @click="choose(o.id)"
        >
          <span class="choice-name">{{ o.name }}</span>
          <span class="badge">{{ roleLabel[o.role] }}</span>
        </button>
      </li>
    </ul>
    <RouterLink class="btn-secondary join" :to="{ name: 'join' }">Gabung sekolah lain</RouterLink>
  </AuthCard>
</template>

<style scoped>
.list {
  list-style: none;
  margin: var(--space-6) 0 var(--space-4);
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.choice {
  width: 100%;
  min-height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border: 2px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
  color: var(--color-ink);
  font: inherit;
  cursor: pointer;
  text-align: left;
}

.choice[aria-current='true'] {
  border-color: var(--color-primary);
}

.choice:focus-visible {
  outline: 3px solid var(--color-primary-strong);
  outline-offset: 2px;
}

.choice-name {
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  font-size: var(--text-lg);
}

.badge {
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-pill);
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
  font-size: var(--text-sm);
  font-weight: var(--weight-bold);
}

.join {
  width: 100%;
}
</style>
