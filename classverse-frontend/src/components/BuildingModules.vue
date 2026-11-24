<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId, watch } from 'vue'
import { errorMessage } from '@/api/client'
import { levels, moduleApi } from '@/api/modules'
import { hasMore, PAGE } from '@/api/page'
import { quizApi } from '@/api/quizzes'
import type { Level, Module, Quiz, QuizAttempt } from '@/api/types'
import FileReader, { type ReadableFile } from './FileReader.vue'
import QuizPlayer from './QuizPlayer.vue'
import LearnPopup from './learn/LearnPopup.vue'
import StepPlayer from './learn/StepPlayer.vue'
import { firstOpen, placeInfo, type PlayerStep } from './learn/place'

// A building's modules in one popup: the catalog (cards per level, plus the building's quizzes) or one module
// being studied. Opening a module swaps the popup content; it never stacks a second popup. Module steps are all
// open and completion can be undone; a quiz step completes on the server when its quiz is passed.
const props = defineProps<{ orgId: string; buildingKey: string; initialLevel?: Level; moduleId?: string; stepId?: string }>()
const emit = defineEmits<{ close: []; changed: [] }>()
const catalogTitle = useId(), playerTitle = useId()
const place = computed(() => placeInfo(props.buildingKey))

const level = ref<Level>(props.initialLevel ?? 'SD')
const modules = ref<Module[]>([])
const quizzes = ref<Quiz[]>([])
const moreModules = ref(false), moreQuizzes = ref(false)
const loading = ref(false)
const error = ref('')
const openId = ref('')
const currentId = ref('')
const saving = ref(false)
const stepError = ref('')
const reading = ref<ReadableFile | null>(null)
const playing = ref<{ id: string; title: string; stepID?: string; moduleID?: string } | null>(null)
let generation = 0
onBeforeUnmount(() => { generation++ })
// A deep link (resume, room pin) opens its module once, after the first load.
let deepLink = props.moduleId

async function load(append = false) {
  const mine = ++generation
  loading.value = true
  error.value = ''
  try {
    const query = { building_key: props.buildingKey, level: level.value, limit: PAGE }
    const moduleOffset = append ? modules.value.length : 0
    const quizOffset = append ? quizzes.value.length : 0
    const [page, quizList] = await Promise.all([
      moduleApi.list(props.orgId, { ...query, offset: moduleOffset }),
      append && !moreQuizzes.value ? Promise.resolve({ items: [] as Quiz[], total: quizzes.value.length }) : quizApi.list(props.orgId, { ...query, offset: quizOffset }),
    ])
    if (mine !== generation) return
    let items = append ? [...modules.value, ...page.items] : page.items
    // A deep link to a module outside the first page fetches just that module.
    if (deepLink && !items.some((m) => m.id === deepLink)) {
      const one = await moduleApi.list(props.orgId, { building_key: props.buildingKey, level: level.value, module_id: deepLink, limit: 1 })
      if (mine !== generation) return
      items = [...one.items, ...items]
    }
    modules.value = items
    quizzes.value = append ? [...quizzes.value, ...quizList.items] : quizList.items
    moreModules.value = hasMore(moduleOffset, page)
    moreQuizzes.value = hasMore(quizOffset, quizList)
    if (deepLink) {
      const target = items.find((m) => m.id === deepLink)
      if (target) openModule(target, props.stepId)
      deepLink = undefined
    }
  } catch (e) {
    if (mine === generation) error.value = errorMessage(e)
  } finally {
    if (mine === generation) loading.value = false
  }
}
watch([() => props.orgId, () => props.buildingKey, level], () => void load(), { immediate: true })

const opened = computed(() => modules.value.find((m) => m.id === openId.value) ?? null)
function openModule(m: Module, stepID?: string) {
  openId.value = m.id
  stepError.value = ''
  currentId.value = m.steps.find((s) => s.id === stepID)?.id ?? firstOpen(m.steps)?.id ?? ''
}
const updateStep = (moduleID: string, stepID: string, patch: Partial<Module['steps'][number]>) => {
  modules.value = modules.value.map((m) => m.id === moduleID
    ? { ...m, steps: m.steps.map((s) => (s.id === stepID ? { ...s, ...patch } : s)) }
    : m)
}
async function setDone(stepID: string, done: boolean) {
  const m = opened.value
  if (!m || saving.value) return
  saving.value = true
  stepError.value = ''
  try {
    await moduleApi.setDone(props.orgId, m.id, stepID, done)
    updateStep(m.id, stepID, { done })
    emit('changed')
  } catch (e) {
    stepError.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}
function openFile(s: PlayerStep) {
  const step = opened.value?.steps.find((x) => x.id === s.id)
  if (step?.file_id) reading.value = { id: step.file_id, name: step.title, kind: step.file_kind ?? 'pdf', position: step.reader_position }
}
function fileClosed(position: number) {
  const id = reading.value?.id
  modules.value = modules.value.map((m) => ({ ...m, steps: m.steps.map((s) => (s.file_id === id ? { ...s, reader_position: position } : s)) }))
  reading.value = null
}
function startQuiz(s: PlayerStep) {
  if (s.quiz_id) playing.value = { id: s.quiz_id, title: s.title, stepID: s.id, moduleID: opened.value?.id }
}
// The server completed the step on a pass (same transaction); mirror it and refresh best scores.
function quizFinished(a: QuizAttempt) {
  const { stepID, moduleID } = playing.value ?? {}
  if (a.passed && stepID && moduleID) {
    updateStep(moduleID, stepID, { done: true })
    emit('changed')
  }
  quizzes.value = quizzes.value.map((z) => z.id === a.quiz_id
    ? { ...z, attempts: z.attempts + 1, best_score: Math.max(z.best_score ?? 0, a.score ?? 0) }
    : z)
}

const minutes = (m: Module) => m.steps.reduce((sum, s) => sum + s.minutes, 0)
const doneOf = (m: Module) => m.steps.filter((s) => s.done).length
const state = (m: Module) => !m.steps.length ? 'empty' : doneOf(m) === m.steps.length ? 'done' : doneOf(m) ? 'going' : 'new'
const stateLabel = { empty: 'Belum ada langkah', new: 'Baru', going: 'Sedang dipelajari', done: 'Selesai' }
const action = { empty: 'Lihat', new: 'Mulai', going: 'Lanjutkan', done: 'Ulangi' }
const dot = { materi: 'M', video: 'V', berkas: 'B', kuis: '?' }
const meta = computed(() => opened.value
  ? [`Gedung ${place.value.name}`, `${opened.value.steps.length} langkah · ${minutes(opened.value)} menit`, 'Boleh loncat langkah']
  : [])
</script>

<template>
  <LearnPopup :labelledby="opened ? playerTitle : catalogTitle" @close="emit('close')">
    <StepPlayer v-if="opened" :title-id="playerTitle" mode="modul" :steps="opened.steps" :current-id="currentId"
      :title="opened.title" :eyebrow="`Modul · ${opened.level}`" :subject="place.subject" :color="place.color" :meta="meta"
      back-label="Semua modul" :busy="saving" :error="stepError"
      @select="currentId = $event" @complete="setDone($event, true)" @undo="setDone($event, false)"
      @open-file="openFile" @start-quiz="startQuiz" @back="openId = ''" />
    <template v-else>
      <header class="head" :style="{ '--tone': place.color }">
        <span class="icon" aria-hidden="true">
          <svg viewBox="0 0 24 24" width="26" height="26"><path d="M4 5a2 2 0 0 1 2-2h13v16H6a2 2 0 0 0-2 2zM4 21V5M8 7h7" /></svg>
        </span>
        <div class="title">
          <p>Gedung · {{ place.subject }}</p>
          <h2 :id="catalogTitle">Modul {{ place.name }}</h2>
        </div>
        <div class="levels" role="group" aria-label="Jenjang">
          <button v-for="l in levels" :key="l" type="button" :aria-pressed="level === l" @click="level = l">{{ l }}</button>
        </div>
      </header>
      <div class="body">
        <p class="section"><strong>Modul jenjang {{ level }}</strong><span>Semua modul terbuka, kerjakan urut atau loncat sesukamu</span></p>
        <p v-if="error" class="alert" role="alert">{{ error }} <button type="button" class="link" @click="load()">Coba lagi</button></p>
        <p v-else-if="loading && !modules.length" role="status" class="muted">Memuat modul…</p>
        <p v-else-if="!modules.length" class="muted">Belum ada modul untuk jenjang ini.</p>
        <ul class="grid">
          <li v-for="m in modules" :key="m.id" class="card">
            <p class="card-top">
              <span class="dots" aria-hidden="true">
                <i v-for="s in m.steps" :key="s.id" :class="[s.kind, { done: s.done }]">{{ s.done ? '✓' : dot[s.kind] }}</i>
              </span>
              <span class="state" :class="state(m)">{{ stateLabel[state(m)] }}</span>
            </p>
            <h3>{{ m.title }}</h3>
            <p v-if="m.description" class="desc">{{ m.description }}</p>
            <p class="card-foot">
              <span>{{ m.steps.length }} langkah · {{ minutes(m) }} menit · {{ doneOf(m) }} dari {{ m.steps.length }} selesai</span>
              <button type="button" :class="state(m) === 'done' ? 'btn-secondary' : 'btn-primary'" :disabled="!m.steps.length" @click="openModule(m)">
                {{ action[state(m)] }}<span class="sr"> {{ m.title }}</span>
              </button>
            </p>
          </li>
        </ul>
        <button v-if="moreModules" type="button" class="btn-secondary more" :disabled="loading" @click="load(true)">Muat lebih banyak</button>
        <section v-if="quizzes.length" class="quizzes" aria-labelledby="quiz-heading">
          <h3 id="quiz-heading">Kuis di gedung ini</h3>
          <article v-for="z in quizzes" :key="z.id" class="quiz-card">
            <span class="quiz-icon" aria-hidden="true"><i /><i /><i /><i /></span>
            <div><strong>{{ z.title }}</strong><small>{{ z.question_count }} soal · {{ z.best_score === null ? 'belum dicoba' : `nilai terbaik ${z.best_score}` }}</small></div>
            <button type="button" class="quiz-play" @click="playing = { id: z.id, title: z.title }">
              {{ z.attempts ? 'Main lagi' : 'Main' }}<span class="sr"> {{ z.title }}</span>
            </button>
          </article>
          <button v-if="moreQuizzes" type="button" class="btn-secondary more" :disabled="loading" @click="load(true)">Muat kuis lainnya</button>
        </section>
      </div>
    </template>
  </LearnPopup>
  <QuizPlayer v-if="playing" :key="playing.id" :org-id="orgId" :quiz-id="playing.id" :title="playing.title"
    :module-id="playing.moduleID" :step-id="playing.stepID" @finished="quizFinished" @close="playing = null" />
  <FileReader v-if="reading" :key="reading.id" :org-id="orgId" :file="reading" @close="fileClosed" />
</template>

<style scoped>
.head { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-4); padding: var(--space-5) calc(var(--space-6) + 60px) var(--space-5) var(--space-6); background: color-mix(in srgb, var(--tone) 9%, white); border-bottom: 1px solid var(--kv-line); }
.icon { flex: none; display: grid; place-items: center; width: 64px; height: 64px; border-radius: 18px; background: var(--tone); color: var(--color-surface); }
.icon svg { fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
.title { flex: 1 1 240px; min-width: 0; }
.title p { margin: 0; font-size: var(--text-sm); font-weight: var(--weight-bold); color: color-mix(in srgb, var(--tone) 80%, black); }
h2 { margin: 0; font-size: 30px; font-weight: 800; overflow-wrap: anywhere; }
.levels { display: flex; gap: var(--space-1); padding: var(--space-1); border: 1px solid var(--kv-line); border-radius: 14px; background: var(--color-surface); }
.levels button { min-height: 40px; min-width: 56px; padding: 0 var(--space-3); border: 0; border-radius: 10px; background: transparent; color: var(--color-ink); font: inherit; font-weight: 800; cursor: pointer; }
.levels button[aria-pressed='true'] { background: var(--tone); color: var(--color-surface); }
.levels button:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
.body { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; gap: var(--space-4); padding: var(--space-6); }
.section { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: baseline; gap: var(--space-2); margin: 0; }
.section strong { font-size: var(--text-xl); font-weight: 800; }
.section span, .muted { margin: 0; color: var(--kv-text-muted); font-size: var(--text-md); font-weight: var(--weight-semibold); }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: var(--space-4); margin: 0; padding: 0; list-style: none; }
.card { display: flex; flex-direction: column; gap: var(--space-2); padding: var(--space-5); border: 1px solid var(--kv-line); border-radius: 18px; box-shadow: 0 3px 0 var(--kv-line); }
.card-top { display: flex; justify-content: space-between; align-items: center; gap: var(--space-2); margin: 0; }
.dots { display: flex; flex-wrap: wrap; gap: 6px; }
.dots i { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 8px; background: var(--kv-info-soft); color: var(--kv-info-text); font-style: normal; font-size: var(--text-sm); font-weight: 800; }
.dots i.kuis { background: var(--kv-arena-soft); color: var(--kv-arena-text); }
.dots i.done { background: var(--kv-learn-done); color: var(--color-surface); }
.state { font-size: var(--text-sm); font-weight: var(--weight-bold); color: var(--kv-text-muted); }
.state.going { color: var(--kv-primary-soft-text); }
.state.done { color: var(--kv-learn-done-ink); }
.card h3 { margin: var(--space-1) 0 0; font-size: var(--text-xl); font-weight: 800; overflow-wrap: anywhere; }
.desc { margin: 0; font-size: 15px; line-height: 1.55; color: var(--kv-text-muted); overflow-wrap: anywhere; }
.card-foot { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-2); margin: auto 0 0; padding-top: var(--space-3); border-top: 1px solid var(--kv-line); font-size: var(--text-sm); font-weight: var(--weight-semibold); color: var(--kv-text-muted); }
.card-foot button { min-height: 40px; }
.more { align-self: center; }
.quizzes { display: grid; gap: var(--space-3); }
.quizzes h3 { margin: var(--space-2) 0 0; font-size: var(--text-xl); font-weight: 800; }
.quiz-card { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-4); padding: var(--space-4) var(--space-5); border-radius: 18px; background: var(--kv-arena-soft); }
.quiz-card > div { flex: 1 1 200px; min-width: 0; display: grid; }
.quiz-card small { color: var(--kv-learn-quiz-ink); font-size: var(--text-md); font-weight: var(--weight-semibold); }
.quiz-icon { flex: none; display: grid; grid-template-columns: 1fr 1fr; gap: 4px; width: 56px; height: 56px; box-sizing: border-box; padding: 10px; border-radius: 14px; background: var(--kv-learn-quiz); }
.quiz-icon i { border-radius: 4px; background: var(--kv-quiz-red); }
.quiz-icon i:nth-child(2) { background: var(--kv-quiz-blue); }
.quiz-icon i:nth-child(3) { background: var(--kv-quiz-yellow); }
.quiz-icon i:nth-child(4) { background: var(--kv-quiz-green); }
.quiz-play { min-height: 44px; padding: 0 var(--space-5); border: 0; border-radius: 12px; background: var(--kv-learn-quiz); color: var(--color-surface); font: inherit; font-weight: 800; cursor: pointer; }
.quiz-play:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
</style>
