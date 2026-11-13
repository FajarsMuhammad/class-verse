<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import { errorMessage } from '@/api/client'
import { kindLabel, levels, moduleApi } from '@/api/modules'
import type { Level, Module, ModuleStep } from '@/api/types'
import FileReader, { type ReadableFile } from './FileReader.vue'
import StepViewer from './StepViewer.vue'

// Learning paths of one building (M1). Progress and locking come with M2; every step is open for now.
const props = defineProps<{ orgId: string; buildingKey: string; buildingName: string }>()

const ids = { level: useId(), list: useId() }
const level = ref<Level>('SD')
const modules = ref<Module[]>([])
const loading = ref(false)
const error = ref('')
const viewing = ref<ModuleStep | null>(null)
const reading = ref<ReadableFile | null>(null)
let generation = 0

async function load() {
  const mine = ++generation
  loading.value = true
  error.value = ''
  try {
    const page = await moduleApi.list(props.orgId, { building_key: props.buildingKey, level: level.value })
    if (mine === generation) modules.value = page.items
  } catch (err) {
    if (mine === generation) error.value = errorMessage(err)
  } finally {
    if (mine === generation) loading.value = false
  }
}
watch([() => props.buildingKey, level], load, { immediate: true })

const minutes = (m: Module) => m.steps.reduce((sum, s) => sum + s.minutes, 0)
const empty = computed(() => !loading.value && !error.value && !modules.value.length)

function open(step: ModuleStep) {
  if (step.kind === 'berkas' && step.file_id) {
    reading.value = { id: step.file_id, name: step.title, kind: step.file_kind ?? 'pdf', position: step.reader_position }
  } else viewing.value = step
}
function closed(position: number) {
  const id = reading.value?.id
  modules.value = modules.value.map((m) => ({
    ...m,
    steps: m.steps.map((s) => (s.file_id === id ? { ...s, reader_position: position } : s)),
  }))
  reading.value = null
}
</script>

<template>
  <section class="paths" :aria-label="`Modul ${buildingName}`">
    <div class="levels" role="group" :aria-labelledby="ids.level">
      <span :id="ids.level" class="sr">Jenjang</span>
      <button v-for="l in levels" :key="l" type="button" class="level" :aria-pressed="level === l" @click="level = l">
        {{ l }}
      </button>
    </div>
    <p v-if="error" class="alert" role="alert">{{ error }} <button type="button" class="link" @click="load()">Coba lagi</button></p>
    <p v-else-if="loading" role="status" class="muted">Memuat modul…</p>
    <p v-else-if="empty" class="muted">Belum ada modul {{ level }} di {{ buildingName }}.</p>
    <div :id="ids.list" class="list">
      <article v-for="m in modules" :key="m.id" class="module" :aria-labelledby="`m-${m.id}`">
        <h3 :id="`m-${m.id}`">{{ m.title }}</h3>
        <p v-if="m.description" class="muted">{{ m.description }}</p>
        <p class="muted small">{{ m.steps.length }} langkah · {{ minutes(m) }} menit</p>
        <ol class="steps">
          <li v-for="(s, i) in m.steps" :key="s.id" :class="['step', s.kind]">
            <span class="dot" aria-hidden="true">{{ i + 1 }}</span>
            <span class="text"><strong>{{ s.title }}</strong><small>{{ kindLabel[s.kind] }} · {{ s.minutes }} mnt</small></span>
            <button type="button" class="btn-secondary open" @click="open(s)">
              {{ s.kind === 'video' ? 'Tonton' : s.file_kind === 'audio' ? 'Dengarkan' : 'Buka' }}<span class="sr"> {{ s.title }}</span>
            </button>
          </li>
        </ol>
        <p v-if="!m.steps.length" class="muted small">Modul ini belum punya langkah.</p>
      </article>
    </div>
    <StepViewer v-if="viewing" :step="viewing" @close="viewing = null" />
    <FileReader v-if="reading" :key="reading.id" :org-id="orgId" :file="reading" @close="closed" />
  </section>
</template>

<style scoped>
.paths {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.levels {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.level {
  min-height: 44px;
  padding: 0 var(--space-3);
  border: none;
  border-radius: var(--radius-pill);
  background: var(--color-surface-alt);
  color: var(--color-ink);
  font: inherit;
  font-weight: var(--weight-semibold);
  cursor: pointer;
}
.level[aria-pressed='true'] {
  background: var(--kv-subject-ipa);
  color: var(--color-surface);
}
.level:focus-visible {
  outline: 3px solid var(--color-primary-strong);
  outline-offset: 2px;
}
.list {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.module {
  padding: var(--space-3);
  border: 1px solid var(--color-border-soft);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}
.module h3 {
  margin: 0;
  font-size: var(--text-base);
}
.muted {
  margin: var(--space-1) 0 0;
  color: var(--color-ink-muted);
}
.small {
  font-size: var(--text-sm);
}
.steps {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: var(--space-3) 0 0;
  padding: 0;
  list-style: none;
}
.step {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.dot {
  flex: none;
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border: 2px solid var(--kv-ink);
  border-radius: 50%;
  background: var(--kv-subject-ipa);
  color: var(--color-surface);
  font-weight: var(--weight-semibold);
}
.step.video .dot {
  background: var(--kv-subject-arena);
}
.step.berkas .dot {
  background: var(--kv-subject-matematika);
}
.text {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  overflow-wrap: anywhere;
}
.text small {
  color: var(--color-ink-muted);
}
.open {
  flex: none;
  min-height: 44px;
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
</style>
