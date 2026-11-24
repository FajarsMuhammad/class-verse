<script setup lang="ts">
import { ref, watch } from 'vue'
import { errorMessage } from '@/api/client'
import { moduleApi } from '@/api/modules'
import type { ModuleResume } from '@/api/types'
import BuildingModules from './BuildingModules.vue'

const props = defineProps<{ orgId: string; refreshKey?: number }>()
const resume = ref<ModuleResume | null>(null)
const error = ref('')
const loading = ref(false)
const opened = ref(false)
let generation = 0
async function load() {
  const mine = ++generation
  loading.value = true
  error.value = ''
  try {
    const res = await moduleApi.resume(props.orgId)
    if (mine === generation) resume.value = res.resume
  } catch (err) {
    if (mine === generation) error.value = errorMessage(err)
  } finally {
    if (mine === generation) loading.value = false
  }
}
watch([() => props.orgId, () => props.refreshKey], load, { immediate: true })
function close() {
  opened.value = false
  void load()
}
</script>

<template>
  <section class="resume" aria-label="Lanjutkan belajar">
    <h3>Lanjutkan belajar</h3>
    <p v-if="error" class="alert" role="alert">{{ error }} <button type="button" class="link" @click="load">Coba lagi</button></p>
    <p v-else-if="loading" class="muted" role="status">Memuat progres…</p>
    <template v-else-if="resume">
      <strong>{{ resume.title }}</strong>
      <p class="muted">{{ resume.done }} dari {{ resume.total }} selesai · {{ resume.level }}</p>
      <p class="muted">Berikutnya: {{ resume.next_title }}</p>
      <button type="button" class="btn-primary" @click="opened = true">Lanjutkan<span class="sr"> {{ resume.title }}</span></button>
    </template>
    <p v-else class="muted">Belum ada modul untuk dilanjutkan. Buka modul dari gedung dan tandai langkah selesai.</p>
    <BuildingModules v-if="opened && resume" :key="orgId" :org-id="orgId" :building-key="resume.building_key"
      :initial-level="resume.level" :module-id="resume.module_id" @close="close" />
  </section>
</template>

<style scoped>
.resume { display: grid; gap: var(--space-2); margin-top: var(--space-4); }
h3, p { margin: 0; }
.muted { color: var(--color-ink-muted); font-size: var(--text-sm); }
</style>
