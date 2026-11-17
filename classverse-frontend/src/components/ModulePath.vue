<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue'
import { errorMessage } from '@/api/client'
import { kindLabel, levels, moduleApi } from '@/api/modules'
import { quizApi } from '@/api/quizzes'
import type { Level, Module, ModuleStep, Quiz, QuizAttempt } from '@/api/types'
import FileReader, { type ReadableFile } from './FileReader.vue'
import QuizPlayer from './QuizPlayer.vue'
import StepViewer from './StepViewer.vue'

// Open catalog: every step remains accessible, regardless of completion order.
const props = defineProps<{ orgId: string; buildingKey: string; buildingName: string; initialLevel?: Level; moduleId?: string; stepId?: string }>()
const emit = defineEmits<{ changed: [] }>()

const ids = { level: useId(), list: useId() }
const level = ref<Level>(props.initialLevel ?? 'SD')
const modules = ref<Module[]>([])
const quizzes = ref<Quiz[]>([])
// The quiz being played; stepID is set when it was opened from a module quiz step.
const playing = ref<{ id: string; title: string; stepID?: string } | null>(null)
const loading = ref(false)
const error = ref('')
const viewing = ref<ModuleStep | null>(null)
const viewingModule = computed(() => modules.value.find((m) => m.steps.some((s) => s.id === viewing.value?.id)))
const reading = ref<ReadableFile | null>(null)
const saving = ref(false)
let generation = 0

async function load() {
  const mine = ++generation
  loading.value = true
  error.value = ''
  try {
    const query = { building_key: props.buildingKey, level: level.value }
    const [page, quizPage] = await Promise.all([moduleApi.list(props.orgId, query), quizApi.list(props.orgId, query)])
    if (mine === generation) {
      modules.value = page.items
      quizzes.value = quizPage.items
      if (props.moduleId) {
        await nextTick()
        document.getElementById(`module-${props.moduleId}`)?.focus()
        const step = page.items.find(m => m.id === props.moduleId)?.steps.find(s => s.id === props.stepId)
        if (step) open(step)
      }
    }
  } catch (err) {
    if (mine === generation) error.value = errorMessage(err)
  } finally {
    if (mine === generation) loading.value = false
  }
}
watch([() => props.orgId, () => props.buildingKey, level], load, { immediate: true })

const minutes = (m: Module) => m.steps.reduce((sum, s) => sum + s.minutes, 0)
const empty = computed(() => !loading.value && !error.value && !modules.value.length && !quizzes.value.length)

async function setDone(m: Module, step: ModuleStep) {
  if (saving.value) return
  saving.value = true
  error.value = ''
  const mine = generation
  try {
    await moduleApi.setDone(props.orgId, m.id, step.id, !step.done)
    if (mine !== generation) return
    modules.value = modules.value.map((item) => item.id === m.id
      ? { ...item, steps: item.steps.map((s) => s.id === step.id ? { ...s, done: !step.done } : s) }
      : item)
    if (viewing.value?.id === step.id) viewing.value = { ...step, done: !step.done }
    emit('changed')
  } catch (err) {
    if (mine === generation) error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}

function open(step: ModuleStep) {
  if (step.kind === 'kuis' && step.quiz_id) {
    viewing.value = null
    playing.value = { id: step.quiz_id, title: step.title, stepID: step.id }
  } else if (step.kind === 'berkas' && step.file_id) {
    viewing.value = null
    reading.value = { id: step.file_id, name: step.title, kind: step.file_kind ?? 'pdf', position: step.reader_position }
  } else viewing.value = step
}
// The server already completed the step on a pass (same transaction); mirror it and refresh best scores.
function quizFinished(a: QuizAttempt) {
  const stepID = playing.value?.stepID
  if (a.passed && stepID) {
    modules.value = modules.value.map((m) => ({ ...m, steps: m.steps.map((s) => (s.id === stepID ? { ...s, done: true } : s)) }))
    emit('changed')
  }
  quizzes.value = quizzes.value.map((z) => z.id === a.quiz_id
    ? { ...z, attempts: z.attempts + 1, best_score: Math.max(z.best_score ?? 0, a.score ?? 0) }
    : z)
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
      <article v-for="m in modules" :id="`module-${m.id}`" :key="m.id" tabindex="-1" class="module" :class="{ selected: moduleId === m.id }" :aria-labelledby="`m-${m.id}`">
        <h3 :id="`m-${m.id}`">{{ m.title }}</h3>
        <p v-if="m.description" class="muted">{{ m.description }}</p>
        <p class="muted small">{{ m.steps.length }} langkah · {{ minutes(m) }} menit</p>
        <p class="muted small" role="status">{{ m.steps.filter((s) => s.done).length }} dari {{ m.steps.length }} selesai</p>
        <ol class="steps">
          <li v-for="(s, i) in m.steps" :key="s.id" :class="['step', s.kind, { done: s.done }]">
            <span class="dot" aria-hidden="true">{{ s.done ? '✓' : s.kind === 'video' ? '▷' : s.kind === 'berkas' ? '▤' : s.kind === 'kuis' ? '?' : '▱' }}<small>{{ i + 1 }}</small></span>
            <span class="text"><strong>{{ s.title }}</strong><small>{{ kindLabel[s.kind] }} · {{ s.minutes }} mnt</small></span>
            <div class="actions">
              <button type="button" class="btn-secondary open" @click="open(s)">
                {{ s.kind === 'video' ? 'Tonton' : s.kind === 'kuis' ? 'Kerjakan' : s.file_kind === 'audio' ? 'Dengarkan' : 'Buka' }}<span class="sr"> {{ s.title }}</span>
              </button>
              <p v-if="s.kind === 'kuis'" class="muted small auto">Selesai otomatis saat lulus</p>
              <button v-else type="button" class="btn-secondary complete" :aria-pressed="s.done" :disabled="saving" @click="setDone(m, s)">
                {{ s.done ? 'Batal selesai' : 'Selesai' }}<span class="sr"> {{ s.title }}</span>
              </button>
            </div>
          </li>
        </ol>
        <p v-if="!m.steps.length" class="muted small">Modul ini belum punya langkah.</p>
      </article>
    </div>
    <section v-if="quizzes.length" class="quizzes" aria-label="Kuis">
      <h3>Kuis</h3>
      <article v-for="z in quizzes" :key="z.id" class="quiz-card">
        <span class="quiz-icon" aria-hidden="true">?</span>
        <div class="text">
          <strong>{{ z.title }}</strong>
          <small>{{ z.question_count }} soal · {{ z.best_score === null ? 'belum dicoba' : `nilai terbaik ${z.best_score}` }}</small>
        </div>
        <button type="button" class="btn-primary" @click="playing = { id: z.id, title: z.title }">
          {{ z.attempts ? 'Main lagi' : 'Main' }}<span class="sr"> {{ z.title }}</span>
        </button>
      </article>
    </section>
    <QuizPlayer v-if="playing" :key="playing.id" :org-id="orgId" :quiz-id="playing.id" :title="playing.title"
      @finished="quizFinished" @close="playing = null" />
    <StepViewer v-if="viewing" :step="viewing" :steps="viewingModule?.steps" :module-title="viewingModule?.title"
      :saving="saving" @select="open" @done="viewingModule && setDone(viewingModule, $event)" @close="viewing = null" />
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
  min-height: var(--touch-min);
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
  padding: var(--space-5);
  border: 1px solid var(--kv-line);
  border-radius: var(--kv-radius-card);
  background: var(--kv-learning-surface);
  box-shadow: var(--kv-shadow-card);
}
.module h3 {
  margin: 0;
  font-size: var(--text-md);
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
  gap: var(--space-6);
  margin: var(--space-5) 0 0;
  padding: 0;
  list-style: none;
}
.module.selected {
  border-color: var(--color-primary-strong);
}
.actions {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.complete {
  min-height: var(--touch-min);
}
.complete[aria-pressed='true'] {
  background: var(--color-primary-soft);
}
.step {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
  text-align: center;
  padding-bottom: var(--space-5);
}
.step:not(:last-child)::after {
  content: '••';
  position: absolute;
  bottom: calc(-1 * var(--space-3));
  color: var(--kv-subject-ipa);
  letter-spacing: var(--space-1);
}
.dot {
  flex: none;
  display: grid;
  place-items: center;
  position: relative;
  width: 76px;
  height: 76px;
  border: 1px solid var(--kv-line);
  box-shadow: var(--kv-shadow-card);
  font-size: var(--text-2xl);
  border-radius: 50%;
  background: var(--kv-star);
  color: var(--kv-ink);
  font-weight: var(--weight-semibold);
}
.step.done .dot { background: var(--kv-subject-ipa); color: var(--kv-surface); }
.dot small { position: absolute; right: -8px; top: -6px; display: grid; place-items: center; width: 26px; height: 26px; border: 1px solid var(--kv-line); border-radius: 50%; background: var(--kv-surface); color: var(--kv-ink); font-size: var(--text-xs); }
@media (min-width: 900px) {
  .step:nth-child(4n + 2) { margin-left: 20%; }
  .step:nth-child(4n) { margin-right: 20%; }
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
  min-height: var(--touch-min);
}
.auto {
  margin: 0;
  align-self: center;
}
.quizzes {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-5);
  border: 1px solid var(--kv-line);
  border-radius: var(--kv-radius-card);
  background: var(--kv-learning-surface);
  box-shadow: var(--kv-shadow-card);
}
.quizzes h3 { margin: 0; font-family: var(--font-display); }
.quiz-card {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--kv-line);
  border-radius: var(--radius-lg);
}
.quiz-icon {
  flex: none;
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  border: 1px solid var(--kv-line);
  border-radius: var(--radius-md);
  background: var(--kv-subject-arena);
  color: var(--kv-surface);
  font-weight: var(--weight-bold);
}
.quiz-card .btn-primary { flex: none; min-height: var(--touch-min); }
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
</style>
