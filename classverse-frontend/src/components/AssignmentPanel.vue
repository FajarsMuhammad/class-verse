<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { assignmentApi, deadline, type Assignment, type AssignmentStep } from '@/api/assignments'
import { errorMessage } from '@/api/client'
import { embedURL, kindLabel } from '@/api/modules'
import RichContent from './RichContent.vue'
import FileReader, { type ReadableFile } from './FileReader.vue'
import QuizPlayer from './QuizPlayer.vue'
const props = defineProps<{ orgId: string }>()
const tasks = ref<Assignment[]>([])
const selected = ref<Assignment | null>(null)
const steps = ref<AssignmentStep[]>([])
const current = ref<AssignmentStep | null>(null)
const reading = ref<ReadableFile | null>(null)
const playing = ref<AssignmentStep | null>(null)
const busy = ref(false)
const error = ref('')
let generation = 0
onBeforeUnmount(() => { generation++ })
async function load() {
  const g = ++generation; busy.value = true; error.value = ''
  try {
    const result = await assignmentApi.list(props.orgId)
    if (g !== generation) return
    tasks.value = result.items
    if (selected.value) {
      selected.value = result.items.find(a => a.id === selected.value?.id) ?? null
      if (selected.value) {
        const result = await assignmentApi.steps(props.orgId, selected.value.id)
        if (g !== generation) return
        steps.value = result.items
        current.value = result.items.find(s => s.id === current.value?.id) ?? null
      }
    }
  } catch (e) { if (g === generation) error.value = errorMessage(e) }
  finally { if (g === generation) busy.value = false }
}
async function open(a: Assignment) {
  selected.value = a; current.value = null; steps.value = []; await load()
}
function view(s: AssignmentStep) {
  if (!s.unlocked || selected.value?.cancelled || busy.value) return
  current.value = s
  if (s.kind === 'berkas' && s.file_id && s.file_kind) {
    reading.value = { id: s.file_id, name: s.title, kind: s.file_kind, position: 0 }
  } else if (s.kind === 'kuis' && s.quiz_id) playing.value = s
}
async function complete(s: AssignmentStep) {
  if (!selected.value || s.done || busy.value) return
  busy.value = true; error.value = ''
  try { await assignmentApi.complete(props.orgId, selected.value.id, s.id); await load() }
  catch (e) { error.value = errorMessage(e) }
  finally { busy.value = false }
}
onMounted(load)
</script>
<template>
  <section class="tasks" aria-labelledby="tasks-heading" :aria-busy="busy">
    <header><h3 id="tasks-heading">Tugas kelas</h3><button type="button" :disabled="busy" @click="load">Muat ulang</button></header>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="busy" role="status">Memuat tugas…</p>
    <p v-if="!tasks.length && !busy">Belum ada tugas kelas.</p>
    <ul class="task-list"><li v-for="a in tasks" :key="a.id">
      <button type="button" :disabled="busy" :aria-pressed="selected?.id === a.id" @click="open(a)">
        <strong>{{ a.title }}</strong><span>{{ a.class_name }} · {{ a.year_label }}</span>
        <span>{{ a.cancelled ? 'Dibatalkan' : `${a.done}/${a.total} selesai` }} · {{ deadline(a.due_at) }}</span>
      </button>
    </li></ul>
    <section v-if="selected" class="detail">
      <h4>{{ selected.title }}</h4>
      <p>Urutan wajib. Progres tugas terpisah dari katalog; nilai dan progres lama tetap tersimpan.</p>
      <p v-if="selected.cancelled">Tugas dibatalkan. Riwayat tetap tersedia.</p>
      <ol><li v-for="s in steps" :key="s.id">
        <button type="button" :disabled="busy || !s.unlocked || selected.cancelled" @click="view(s)">{{ s.title }} · {{ s.done ? 'Selesai' : s.unlocked ? kindLabel[s.kind] : 'Terkunci' }}</button>
      </li></ol>
      <div v-if="current" class="content">
        <h4>{{ current.title }}</h4>
        <RichContent v-if="current.rich_content" :content="current.rich_content" />
        <p v-else-if="current.body" class="body">{{ current.body }}</p>
        <iframe v-if="current.kind === 'video' && current.video_id" :src="embedURL(current.video_id)" :title="current.title"
          sandbox="allow-scripts allow-same-origin allow-presentation" allow="encrypted-media; picture-in-picture; fullscreen" referrerpolicy="strict-origin-when-cross-origin" />
        <p v-if="current.kind === 'kuis'">Lulus kuis melalui tugas ini. Hasil kuis katalog tidak menyelesaikan tugas.</p>
        <button v-if="current.kind !== 'kuis' && !current.done && !selected.cancelled" type="button" :disabled="busy" @click="complete(current)">Tandai selesai</button>
      </div>
    </section>
    <FileReader v-if="reading" :org-id="orgId" :file="reading" @close="reading = null" />
    <QuizPlayer v-if="playing?.quiz_id" :org-id="orgId" :quiz-id="playing.quiz_id" :assignment-step-id="playing.id" :title="playing.title" @close="playing = null" @finished="load" />
  </section>
</template>
<style scoped>
.tasks { padding: var(--space-3) 0; background: var(--color-surface); border-block: 1px solid var(--kv-line); min-width: 0; font-size: var(--text-md); }
header { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-2); }
h3, h4 { font-family: var(--font-display); margin-block: var(--space-2); }
button { min-height: var(--touch-min); font: inherit; color: var(--color-ink); background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: var(--space-2); cursor: pointer; max-width: 100%; overflow-wrap: anywhere; }
button:disabled { opacity: .6; cursor: not-allowed; }
.task-list { margin: var(--space-2) 0; padding: 0; list-style: none; }
.task-list li { margin: 0; }
.task-list li + li { border-top: 1px solid var(--kv-line); }
.task-list button { display: flex; flex-direction: column; gap: var(--space-1); text-align: left; width: 100%; margin: 0; padding: var(--space-3) var(--space-2); border: 0; border-radius: var(--radius-sm); font-size: var(--text-sm); }
.task-list button:hover:not(:disabled) { background: var(--color-surface-alt); }
button:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: -3px; }
ol { padding-left: var(--space-5); }
li { margin-block: var(--space-2); }
.body { white-space: pre-wrap; overflow-wrap: anywhere; }
iframe { width: 100%; aspect-ratio: 16/9; border: 0; }
.detail { border-top: 1px solid var(--color-border); }
</style>
