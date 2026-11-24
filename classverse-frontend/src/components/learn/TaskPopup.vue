<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId, watch } from 'vue'
import { assignmentApi, deadline, type Assignment, type AssignmentStep } from '@/api/assignments'
import { errorMessage } from '@/api/client'
import { hasMore } from '@/api/page'
import FileReader, { type ReadableFile } from '../FileReader.vue'
import QuizPlayer from '../QuizPlayer.vue'
import LearnPopup from './LearnPopup.vue'
import StepPlayer from './StepPlayer.vue'
import { firstOpen, placeInfo, type PlayerStep } from './place'

// Class tasks in one popup: the full list, or one task being worked on. The server owns the rules: steps
// unlock in order, completion is idempotent, a quiz step completes only when its quiz is passed through the task.
const props = defineProps<{ orgId: string; open?: Assignment | null }>()
const emit = defineEmits<{ close: []; changed: [] }>()
const listTitle = useId(), playerTitle = useId()

type Filter = 'all' | 'todo' | 'done' | 'cancelled'
const filters: { key: Filter; label: string }[] = [
  { key: 'all', label: 'Semua' }, { key: 'todo', label: 'Belum selesai' }, { key: 'done', label: 'Selesai' }, { key: 'cancelled', label: 'Dibatalkan' },
]
const isDone = (a: Assignment) => !a.cancelled && a.total > 0 && a.done >= a.total
const inFilter = (a: Assignment, f: Filter) =>
  f === 'all' || (f === 'cancelled' ? a.cancelled : f === 'done' ? isDone(a) : !a.cancelled && !isDone(a))

const tasks = ref<Assignment[]>([])
const more = ref(false)
const search = ref('')
const filter = ref<Filter>('all')
const listBusy = ref(false)
const listError = ref('')
const shown = computed(() => tasks.value.filter((a) => inFilter(a, filter.value)))
const count = (f: Filter) => tasks.value.filter((a) => inFilter(a, f)).length

const task = ref<Assignment | null>(props.open ?? null)
const steps = ref<AssignmentStep[]>([])
const currentId = ref('')
const busy = ref(false)
const error = ref('')
const reading = ref<ReadableFile | null>(null)
const playing = ref<AssignmentStep | null>(null)
let generation = 0
onBeforeUnmount(() => { generation++ })

// The list is searched and paged by the server; the status tabs filter what is loaded.
async function loadList(append = false) {
  const mine = ++generation
  listBusy.value = true
  listError.value = ''
  try {
    const offset = append ? tasks.value.length : 0
    const page = await assignmentApi.list(props.orgId, offset, search.value.trim())
    if (mine !== generation) return
    tasks.value = append ? [...tasks.value, ...page.items] : page.items
    more.value = hasMore(offset, page)
  } catch (e) {
    if (mine === generation) listError.value = errorMessage(e)
  } finally {
    if (mine === generation) listBusy.value = false
  }
}
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, () => { clearTimeout(timer); timer = setTimeout(() => void loadList(), 300) })
onBeforeUnmount(() => clearTimeout(timer))

async function loadSteps(keepCurrent: boolean) {
  if (!task.value) return
  const mine = ++generation
  busy.value = true
  error.value = ''
  try {
    const result = await assignmentApi.steps(props.orgId, task.value.id)
    if (mine !== generation) return
    steps.value = result.items
    const kept = keepCurrent && result.items.some((s) => s.id === currentId.value)
    if (!kept) currentId.value = firstOpen(result.items)?.id ?? ''
    task.value = { ...task.value, done: result.items.filter((s) => s.done).length, total: result.items.length }
  } catch (e) {
    if (mine === generation) error.value = errorMessage(e)
  } finally {
    if (mine === generation) busy.value = false
  }
}
function openTask(a: Assignment) {
  task.value = a
  steps.value = []
  currentId.value = ''
  void loadSteps(false)
}
function backToList() {
  task.value = null
  void loadList()
}
if (task.value) void loadSteps(false)
else void loadList()

async function complete(id: string) {
  if (!task.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    await assignmentApi.complete(props.orgId, task.value.id, id)
    emit('changed')
    busy.value = false
    await loadSteps(true)
  } catch (e) {
    error.value = errorMessage(e)
    busy.value = false
  }
}
function openFile(s: PlayerStep) {
  if (s.file_id) reading.value = { id: s.file_id, name: s.title, kind: s.file_kind ?? 'pdf', position: 0 }
}
function startQuiz(s: PlayerStep) {
  playing.value = steps.value.find((x) => x.id === s.id) ?? null
}
// The server completed the step on a pass; reload so the next step unlocks.
function quizFinished() {
  emit('changed')
  void loadSteps(true)
}

const kinds = (a: Assignment) => a.total ? `${a.total} langkah` : 'Belum ada langkah'
const status = (a: Assignment) => a.cancelled ? 'Dibatalkan' : isDone(a) ? 'Selesai' : a.done ? 'Sedang dikerjakan' : 'Belum dimulai'
const action = (a: Assignment) => a.cancelled || isDone(a) ? 'Lihat' : a.done ? 'Lanjutkan' : 'Mulai'
const place = computed(() => placeInfo(task.value?.building_key ?? ''))
const meta = computed(() => task.value
  ? [`${task.value.class_name} · ${task.value.year_label}`, deadline(task.value.due_at), task.value.cancelled ? 'Dibatalkan' : 'Urutan wajib']
  : [])
</script>

<template>
  <LearnPopup :labelledby="task ? playerTitle : listTitle" @close="emit('close')">
    <StepPlayer v-if="task" :title-id="playerTitle" mode="tugas" :steps="steps" :current-id="currentId" :title="task.title"
      eyebrow="Tugas kelas" :subject="place.subject" :color="place.color" :meta="meta" back-label="Semua tugas"
      :busy="busy" :readonly="task.cancelled" :error="error"
      @select="currentId = $event" @complete="complete" @open-file="openFile" @start-quiz="startQuiz" @back="backToList" />
    <template v-else>
      <header class="list-head">
        <div>
          <h2 :id="listTitle">Tugas kelas</h2>
          <p class="summary">{{ count('todo') }} aktif · {{ count('done') }} selesai · {{ count('cancelled') }} dibatalkan</p>
        </div>
        <label class="search">
          <span class="sr">Cari tugas</span>
          <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
          <input v-model="search" type="search" placeholder="Cari tugas" />
        </label>
      </header>
      <div class="list-body">
        <div class="tabs" role="group" aria-label="Saring tugas">
          <button v-for="f in filters" :key="f.key" type="button" :aria-pressed="filter === f.key" @click="filter = f.key">
            {{ f.label }} <span>{{ count(f.key) }}</span>
          </button>
        </div>
        <p v-if="listError" class="alert" role="alert">{{ listError }} <button type="button" class="link" @click="loadList()">Coba lagi</button></p>
        <p v-else-if="listBusy && !tasks.length" role="status" class="muted">Memuat tugas…</p>
        <p v-else-if="!shown.length" class="muted">{{ search ? 'Tidak ada tugas yang cocok.' : 'Belum ada tugas di bagian ini.' }}</p>
        <ul class="grid">
          <li v-for="a in shown" :key="a.id" class="card" :class="{ cancelled: a.cancelled }" :style="{ '--tone': placeInfo(a.building_key).color }">
            <p class="card-top"><span class="chip">{{ placeInfo(a.building_key).subject }}</span><span class="status">{{ status(a) }}</span></p>
            <h3>{{ a.title }}</h3>
            <p class="muted">{{ kinds(a) }} · {{ a.class_name }} · {{ a.level }}</p>
            <p class="bar-label"><span>Progres</span><span>{{ a.done }} dari {{ a.total }}</span></p>
            <div class="bar" role="progressbar" :aria-label="`Progres ${a.title}`" aria-valuemin="0" :aria-valuemax="a.total" :aria-valuenow="a.done">
              <span :style="{ width: `${a.total ? (a.done / a.total) * 100 : 0}%` }" />
            </div>
            <p class="card-foot">
              <span>{{ a.cancelled ? 'Riwayat tetap tersedia' : deadline(a.due_at) }}</span>
              <button type="button" :class="action(a) === 'Lihat' ? 'btn-secondary' : 'btn-primary'" @click="openTask(a)">
                {{ action(a) }}<span class="sr"> {{ a.title }}</span>
              </button>
            </p>
          </li>
        </ul>
        <button v-if="more" type="button" class="btn-secondary more" :disabled="listBusy" @click="loadList(true)">Muat lebih banyak</button>
      </div>
    </template>
  </LearnPopup>
  <FileReader v-if="reading" :key="reading.id" :org-id="orgId" :file="reading" @close="reading = null" />
  <QuizPlayer v-if="playing?.quiz_id" :key="playing.id" :org-id="orgId" :quiz-id="playing.quiz_id" :assignment-step-id="playing.id"
    :title="playing.title" @close="playing = null" @finished="quizFinished" />
</template>

<style scoped>
.list-head { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-4); padding: var(--space-5) calc(var(--space-6) + 60px) var(--space-5) var(--space-6); border-bottom: 1px solid var(--kv-line); }
.list-head > div { flex: 1 1 260px; }
h2 { margin: 0; font-size: 30px; font-weight: 800; }
.summary { margin: var(--space-1) 0 0; color: var(--kv-text-muted); font-size: var(--text-md); font-weight: var(--weight-semibold); }
.search { display: flex; align-items: center; gap: var(--space-2); width: min(290px, 100%); height: 44px; box-sizing: border-box; padding: 0 var(--space-3); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); }
.search svg { flex: none; fill: none; stroke: var(--kv-text-muted); stroke-width: 2; stroke-linecap: round; }
.search input { flex: 1; min-width: 0; border: 0; font: inherit; font-size: var(--text-base); background: transparent; color: inherit; }
.search:focus-within { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
.search input:focus { outline: none; }
.list-body { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; gap: var(--space-5); padding: var(--space-5) var(--space-6); }
.tabs { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.tabs button { min-height: 40px; padding: 0 var(--space-4); border: 1px solid var(--kv-line); border-radius: var(--radius-pill); background: var(--color-surface); color: var(--color-ink); font: inherit; font-size: var(--text-md); font-weight: var(--weight-bold); cursor: pointer; }
.tabs button[aria-pressed='true'] { border-color: var(--kv-primary); background: var(--kv-primary); color: var(--color-surface); }
.tabs button span { font-weight: var(--weight-semibold); opacity: .8; }
.tabs button:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: var(--space-4); margin: 0; padding: 0; list-style: none; }
.card { display: flex; flex-direction: column; gap: var(--space-2); padding: var(--space-5); border: 1px solid var(--kv-line); border-radius: 18px; box-shadow: 0 3px 0 var(--kv-line); }
.card.cancelled { opacity: .7; }
.card-top { display: flex; justify-content: space-between; gap: var(--space-2); margin: 0; font-size: var(--text-sm); font-weight: var(--weight-bold); }
.chip { padding: 3px 10px; border-radius: var(--radius-pill); background: color-mix(in srgb, var(--tone) 12%, white); color: color-mix(in srgb, var(--tone) 80%, black); }
.status { color: var(--kv-primary-soft-text); }
.card h3 { margin: var(--space-1) 0 0; font-size: var(--text-xl); font-weight: 800; overflow-wrap: anywhere; }
.muted { margin: 0; color: var(--kv-text-muted); font-size: var(--text-md); }
.bar-label { display: flex; justify-content: space-between; margin: var(--space-2) 0 0; font-size: var(--text-sm); font-weight: var(--weight-bold); }
.bar-label span:first-child { color: var(--kv-text-muted); }
.bar { height: 8px; border-radius: var(--radius-pill); background: var(--kv-line); overflow: hidden; }
.bar span { display: block; height: 100%; border-radius: inherit; background: var(--tone); }
.card-foot { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-2); margin: var(--space-2) 0 0; padding-top: var(--space-3); border-top: 1px solid var(--kv-line); font-size: var(--text-sm); font-weight: var(--weight-semibold); color: var(--kv-text-muted); }
.card-foot button { min-height: 40px; }
.more { align-self: center; }
</style>
