<script setup lang="ts">
import { onMounted, useTemplateRef } from 'vue'
import ModulePath from './ModulePath.vue'
import type { Level } from '@/api/types'

// Modal wrapper so a participant browses a building's modules without leaving the map.
defineProps<{ orgId: string; buildingKey: string; buildingName: string; initialLevel?: Level; moduleId?: string; stepId?: string }>()
const emit = defineEmits<{ close: [] }>()
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
onMounted(() => dialog.value?.showModal())
</script>

<template>
  <dialog ref="dialog" class="modules" aria-labelledby="modules-title" @close="emit('close')">
    <header class="head">
      <h2 id="modules-title">Modul {{ buildingName }}</h2>
      <button type="button" class="btn-secondary close" aria-label="Tutup modul" @click="dialog?.close()">×</button>
    </header>
    <ModulePath :org-id="orgId" :building-key="buildingKey" :building-name="buildingName"
      :initial-level="initialLevel" :module-id="moduleId" :step-id="stepId" />
  </dialog>
</template>

<style scoped>
.modules {
  box-sizing: border-box;
  width: min(100% - var(--space-6), 1100px);
  max-height: calc(100dvh - var(--space-6));
  padding: var(--space-5);
  border: 3px solid var(--kv-ink);
  box-shadow: 0 8px 0 var(--kv-ink);
  border-radius: var(--radius-2xl);
  background: var(--kv-learning-ground);
  background-image: radial-gradient(var(--kv-learning-dot) 1px, transparent 1px);
  background-size: 24px 24px;
  color: var(--color-ink);
}
.modules::backdrop {
  background: var(--color-ink);
  opacity: 0.4;
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  margin-bottom: var(--space-5);
  padding: var(--space-5);
  border: 3px solid var(--kv-ink);
  border-radius: var(--radius-2xl);
  background: var(--kv-subject-ipa);
  color: var(--kv-surface);
}
h2 {
  margin: 0;
  font: var(--weight-semibold) var(--text-xl)/var(--line-tight) var(--font-display);
}
.close {
  flex: none;
  min-width: 44px;
  padding: 0;
}
</style>
