<script setup lang="ts">
import { onMounted, useTemplateRef } from 'vue'
import ModulePath from './ModulePath.vue'

// Modal wrapper so a participant browses a building's modules without leaving the map.
defineProps<{ orgId: string; buildingKey: string; buildingName: string }>()
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
    <ModulePath :org-id="orgId" :building-key="buildingKey" :building-name="buildingName" />
  </dialog>
</template>

<style scoped>
.modules {
  box-sizing: border-box;
  width: min(100% - var(--space-6), 560px);
  max-height: calc(100dvh - var(--space-6));
  padding: var(--space-5);
  border: 2px solid var(--color-border);
  border-radius: var(--radius-2xl);
  background: var(--color-surface);
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
  margin-bottom: var(--space-3);
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
