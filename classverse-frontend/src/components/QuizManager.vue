<script setup lang="ts">
import { computed, nextTick, onMounted, ref, useId } from 'vue'
import { ApiError, errorMessage } from '@/api/client'
import { levels } from '@/api/modules'
import { quizApi, shapes } from '@/api/quizzes'
import type { Level, QuestionInput, Quiz, QuizInput, QuizQuestion } from '@/api/types'
import { fetchMap, type Building } from '@/realtime/mapdata'

// Kelola → Kuis (Q1-01): staff author four-option quizzes per building and level. Started attempts keep
// their own copy of the questions, so editing here never changes a result.
const props = defineProps<{ orgId: string }>()

const id = useId()
const quizzes = ref<Quiz[]>([])
const buildings = ref<Building[]>([])
const editing = ref<string | null>(null) // quiz id or 'new'
const form = ref<QuizInput>(blank())
const questionFor = ref<string | null>(null) // quiz id whose question form is open
const questionEditing = ref<string | null>(null) // question id or 'new'
const question = ref<QuestionInput>(blankQuestion())
const fields = ref<Record<string, string>>({})
const busy = ref(false)
const message = ref('')
const error = ref('')
const levelFilter = ref<Level | ''>('')
const selected = ref<string | null>(null)
const visible = computed(() => quizzes.value.filter((item) => !levelFilter.value || item.level === levelFilter.value))
const active = computed(() => quizzes.value.find((item) => item.id === selected.value))
const preview = ref(false)
const editorHeading = ref<HTMLElement | null>(null)
async function focusEditor() {
  await nextTick()
  editorHeading.value?.focus()
}
function openItem(item: Quiz) {
  selected.value = item.id
  preview.value = false
  editing.value = null
  questionFor.value = null
  fields.value = {}
  error.value = ''
  void focusEditor()
}
function back() {
  if ((editing.value || questionFor.value) && !window.confirm('Tutup form tanpa menyimpan perubahan?')) return
  selected.value = null
  editing.value = null
  questionFor.value = null
  preview.value = false
}


function blank(): QuizInput {
  return { building_key: '', level: 'SD', title: '', description: '', pass_score: 60, published: false }
}
function blankQuestion(): QuestionInput {
  return { prompt: '', options: ['', '', '', ''], answer: 0, explanation: '' }
}
const choices = computed(() => buildings.value.filter((b) => !b.staffOnly))
const buildingName = (key: string) => buildings.value.find((b) => b.key === key)?.name ?? key

async function load() {
  try {
    quizzes.value = (await quizApi.list(props.orgId, { drafts: true })).items
  } catch (err) {
    error.value = errorMessage(err)
  }
}

async function run(fn: () => Promise<unknown>, done: string) {
  busy.value = true
  error.value = ''
  message.value = ''
  fields.value = {}
  try {
    await fn()
    await load()
    message.value = done
    return true
  } catch (err) {
    if (err instanceof ApiError) fields.value = err.fields
    error.value = errorMessage(err)
    return false
  } finally {
    busy.value = false
  }
}

function edit(z?: Quiz) {
  selected.value = z?.id ?? null
  preview.value = false
  questionFor.value = null
  editing.value = z?.id ?? 'new'
  fields.value = {}
  form.value = z
    ? { building_key: z.building_key, level: z.level, title: z.title, description: z.description, pass_score: z.pass_score, published: z.published }
    : { ...blank(), level: levelFilter.value || 'SD', building_key: choices.value[0]?.key ?? '' }
}
async function save() {
  const body = form.value
  const ok = await run(
    async () => {
      const saved = editing.value === 'new'
        ? await quizApi.create(props.orgId, { ...body, published: false })
        : await quizApi.update(props.orgId, editing.value!, body)
      selected.value = saved.id
    },
    `${body.title.trim()} ${body.published ? 'terbit' : 'disimpan sebagai draf'}.`,
  )
  if (ok) {
    editing.value = null
    const item = active.value
    if (item && !item.questions?.length) editQuestion(item)
    void focusEditor()
  }
}
async function setPublished(item: Quiz) {
  const { building_key, level, title, description } = item
  await run(() => quizApi.update(props.orgId, item.id, {
    building_key, level, title, description, pass_score: item.pass_score, published: !item.published,
  }), item.published ? 'Kembali menjadi draf.' : 'Kuis diterbitkan.')
}
async function archive(z: Quiz) {
  if (!window.confirm(`Arsipkan kuis ${z.title}? Peserta tidak bisa memainkannya lagi; riwayat nilai tetap tersimpan.`)) return
  if (await run(() => quizApi.archive(props.orgId, z.id), `${z.title} diarsipkan.`)) selected.value = null
}

function editQuestion(z: Quiz, q?: QuizQuestion) {
  editing.value = null
  questionFor.value = z.id
  questionEditing.value = q?.id ?? 'new'
  fields.value = {}
  question.value = q
    ? { prompt: q.prompt, options: [...q.options], answer: q.answer, explanation: q.explanation }
    : blankQuestion()
}
async function saveQuestion() {
  const qid = questionFor.value!
  const body = { ...question.value, options: [...question.value.options] }
  const ok = await run(
    () => (questionEditing.value === 'new'
      ? quizApi.addQuestion(props.orgId, qid, body)
      : quizApi.updateQuestion(props.orgId, qid, questionEditing.value!, body)),
    'Soal disimpan.',
  )
  if (ok) questionFor.value = null
}
async function removeQuestion(z: Quiz, q: QuizQuestion) {
  if (!window.confirm(`Hapus soal "${q.prompt}"? Nilai yang sudah ada tidak berubah.`)) return
  await run(() => quizApi.deleteQuestion(props.orgId, z.id, q.id), 'Soal dihapus.')
}
async function move(z: Quiz, index: number, delta: -1 | 1) {
  const order = (z.questions ?? []).map((q) => q.id)
  const [moved] = order.splice(index, 1)
  order.splice(index + delta, 0, moved!)
  await run(() => quizApi.reorder(props.orgId, z.id, order), `Urutan ${z.title} disimpan.`)
}

onMounted(async () => {
  await Promise.all([
    load(),
    fetchMap('campus').then((map) => (buildings.value = map.buildings)).catch((err) => (error.value = errorMessage(err))),
  ])
})
</script>

<template>
  <section class="manager authoring" :aria-labelledby="`${id}-title`">
    <header v-if="!selected && !editing" class="head">
      <h2 :id="`${id}-title`" class="title">Kuis</h2>
      <button v-if="!editing" type="button" class="btn-primary" :disabled="!choices.length" @click="edit()">+ Kuis baru</button>
    </header>
    <p v-if="!selected && !editing" class="sub">
      Soal pilihan ganda tepat 4 pilihan. Server menilai; kunci dan penjelasan terlihat peserta setelah kuis selesai.
      Peserta boleh mengulang; Rapor memakai nilai tertinggi.
    </p>
    <p v-if="message" class="notice" role="status">{{ message }}</p>
    <p v-if="error" class="alert" role="alert">{{ error }} {{ !editing ? fields.questions : '' }}</p>

    <header v-if="selected || editing" class="editor-head">
      <button type="button" class="btn-secondary" data-action="back" :disabled="busy" @click="back">Kembali ke daftar</button>
      <h3 ref="editorHeading" class="editor-title" tabindex="-1">{{ active?.title || 'Kuis baru' }}</h3>
      <span v-if="active" class="chip">{{ active.published ? 'Terbit' : 'Draf' }}</span>
    </header>
    <div v-if="active && !editing && !questionFor" class="editor-actions actions">
      <button type="button" class="btn-secondary" :aria-pressed="preview" @click="preview = !preview">{{ preview ? 'Kembali menyusun' : 'Pratinjau isi' }}</button>
      <button type="button" class="btn-secondary" :disabled="busy" @click="edit(active)">Pengaturan</button>
      <button type="button" class="btn-primary" :disabled="busy || (!active.published && !active.question_count)" @click="setPublished(active)">{{ active.published ? 'Jadikan draf' : 'Terbitkan' }}</button>
    </div>
    <p v-if="active && !active.published && !active.question_count" class="hint">Tambahkan minimal satu soal sebelum menerbitkan.</p>
    <div v-if="!selected && !editing" class="catalog">
      <label :for="`${id}-filter`">Jenjang</label>
      <select :id="`${id}-filter`" v-model="levelFilter" aria-label="Filter jenjang">
        <option value="">Semua jenjang</option>
        <option v-for="level in levels" :key="level" :value="level">{{ level }}</option>
      </select>
      <p v-if="!visible.length" class="hint">{{ quizzes.length ? 'Belum ada kuis untuk jenjang ini.' : 'Belum ada kuis. Buat kuis pertama.' }}</p>
      <ul class="catalog-list">
        <li v-for="item in visible" :key="item.id" class="catalog-row">
          <div><strong>{{ item.title }}</strong>
            <p class="meta">{{ buildingName(item.building_key) }} · {{ item.level }} · {{ item.question_count }} soal · lulus {{ item.pass_score }}</p>
          </div>
          <span class="chip">{{ item.published ? 'Terbit' : 'Draf' }}</span>
          <button type="button" class="btn-secondary" :aria-label="`Buka ${item.title}`" @click="openItem(item)">Buka</button>
        </li>
      </ul>
    </div>
    <form v-if="editing" class="form" novalidate @submit.prevent="save">
      <h3 class="form-title">{{ editing === 'new' ? 'Kuis baru' : 'Ubah kuis' }}</h3>
      <div class="grid">
        <label :for="`${id}-b`">Gedung</label>
        <select :id="`${id}-b`" v-model="form.building_key" :aria-invalid="!!fields.building_key">
          <option v-for="b in choices" :key="b.key" :value="b.key">{{ b.name }}</option>
        </select>
        <p v-if="fields.building_key" class="field-error">{{ fields.building_key }}</p>
        <label :for="`${id}-l`">Jenjang</label>
        <select :id="`${id}-l`" v-model="form.level">
          <option v-for="l in levels" :key="l" :value="l">{{ l }}</option>
        </select>
        <label :for="`${id}-t`">Judul</label>
        <input :id="`${id}-t`" v-model="form.title" maxlength="120" :aria-invalid="!!fields.title" />
        <p v-if="fields.title" class="field-error">{{ fields.title }}</p>
        <label :for="`${id}-d`">Deskripsi</label>
        <textarea :id="`${id}-d`" v-model="form.description" maxlength="500" rows="2" />
        <label :for="`${id}-p`">Nilai lulus</label>
        <input :id="`${id}-p`" v-model.number="form.pass_score" type="number" min="1" max="100" inputmode="numeric"
          :aria-invalid="!!fields.pass_score" />
        <p :class="fields.pass_score ? 'field-error' : 'hint'">{{ fields.pass_score || 'Nilai 0–100. Lulus juga menyelesaikan langkah modul yang memakai kuis ini.' }}</p>
      </div>
      <p v-if="editing === 'new'" class="hint">Disimpan sebagai draf. Tambahkan isi, lalu terbitkan.</p>
      <p v-if="fields.questions" class="field-error">{{ fields.questions }}</p>
      <div class="actions">
        <button type="submit" class="btn-primary" :disabled="busy">Simpan</button>
        <button type="button" class="btn-secondary" :disabled="busy" @click="editing = null">Batal</button>
      </div>
    </form>

    <article v-for="z in active && !editing ? [active] : []" :key="z.id" class="quiz" :aria-labelledby="`${id}-${z.id}`">
      <div class="quiz-head">
        <div>
          <h3 :id="`${id}-${z.id}`">{{ z.title }}</h3>
          <p class="meta">
            {{ buildingName(z.building_key) }} · {{ z.level }} · {{ z.question_count }} soal · lulus {{ z.pass_score }}
            <span v-if="!z.published" class="chip">Draf</span>
          </p>
        </div>
        <div v-if="!preview" class="actions">
          <button type="button" class="btn-secondary" :disabled="busy" @click="archive(z)">Arsipkan<span class="sr"> {{ z.title }}</span></button>
        </div>
      </div>
      <ol v-if="!questionFor" class="questions">
        <li v-for="(q, i) in z.questions ?? []" :key="q.id">
          <div class="question-text">
            <strong>{{ i + 1 }}. {{ q.prompt }}</strong>
            <ul v-if="preview" class="options">
              <li v-for="(o, oi) in q.options" :key="oi" :class="{ correct: oi === q.answer }">
                <span aria-hidden="true">{{ shapes[oi]!.symbol }}</span> {{ o }}<span v-if="oi === q.answer"> (jawaban benar)</span>
              </li>
            </ul>
          </div>
          <span v-if="!preview" class="actions">
            <button type="button" class="btn-secondary icon" :disabled="busy || i === 0" :aria-label="`Naikkan ${q.prompt}`" @click="move(z, i, -1)">↑</button>
            <button type="button" class="btn-secondary icon" :disabled="busy || i === (z.questions?.length ?? 0) - 1" :aria-label="`Turunkan ${q.prompt}`" @click="move(z, i, 1)">↓</button>
            <button type="button" class="btn-secondary" :disabled="busy" @click="editQuestion(z, q)">Ubah<span class="sr"> soal {{ q.prompt }}</span></button>
            <button type="button" class="btn-secondary" :disabled="busy" :aria-label="`Hapus soal ${q.prompt}`" @click="removeQuestion(z, q)">Hapus</button>
          </span>
        </li>
      </ol>
      <form v-if="questionFor === z.id" class="form question-form" novalidate @submit.prevent="saveQuestion">
        <h4 class="form-title">{{ questionEditing === 'new' ? 'Soal baru' : 'Ubah soal' }}</h4>
        <label :for="`${id}-qp`">Pertanyaan</label>
        <textarea :id="`${id}-qp`" v-model="question.prompt" maxlength="500" rows="2" :aria-invalid="!!fields.prompt" />
        <p v-if="fields.prompt" class="field-error">{{ fields.prompt }}</p>
        <fieldset class="options-edit">
          <legend>Pilihan (tandai jawaban benar)</legend>
          <div v-for="(_, oi) in question.options" :key="oi" class="option-row">
            <input :id="`${id}-qa${oi}`" v-model="question.answer" type="radio" :name="`${id}-answer`" :value="oi"
              :aria-label="`Jawaban benar: pilihan ${String.fromCharCode(65 + oi)}`" />
            <label :for="`${id}-qo${oi}`" class="shape-label"><span aria-hidden="true">{{ shapes[oi]!.symbol }}</span> {{ String.fromCharCode(65 + oi) }}</label>
            <input :id="`${id}-qo${oi}`" v-model="question.options[oi]" type="text" maxlength="200" :aria-invalid="!!fields.options" />
          </div>
        </fieldset>
        <p v-if="fields.options" class="field-error">{{ fields.options }}</p>
        <p v-if="fields.answer" class="field-error">{{ fields.answer }}</p>
        <details class="feedback"><summary>Penjelasan jawaban (opsional)</summary>
          <label :for="`${id}-qe`">Tampil setelah kuis selesai</label>
          <textarea :id="`${id}-qe`" v-model="question.explanation" maxlength="500" rows="2" />
        </details>
        <p v-if="fields.questions" class="field-error">{{ fields.questions }}</p>
        <div class="actions">
          <button type="submit" class="btn-primary" :disabled="busy">Simpan soal</button>
          <button type="button" class="btn-secondary" :disabled="busy" @click="questionFor = null">Batal</button>
        </div>
      </form>
      <button v-else-if="!preview" type="button" class="btn-primary add" :disabled="busy" @click="editQuestion(z)">+ Soal<span class="sr"> di {{ z.title }}</span></button>
    </article>
  </section>
</template>

<style scoped>
.manager {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  margin-top: var(--space-6);
  padding: var(--space-5);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}
.head,
.quiz-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
}
.title,
.sub,
.form-title,
.quiz h3,
.meta {
  margin: 0;
}
.title {
  font-family: var(--font-display);
  font-size: var(--text-xl);
}
.sub,
.meta,
.hint {
  color: var(--color-ink-muted);
}
.quiz {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-4);
  border: 1px solid var(--color-border-soft);
  border-radius: var(--radius-lg);
}
.chip {
  margin-left: var(--space-1);
  padding: 0 var(--space-2);
  border-radius: var(--radius-pill);
  background: var(--color-accent-soft);
  color: var(--color-accent-ink);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
}
.questions {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}
.questions > li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  background: var(--color-surface-alt);
}
.question-text {
  min-width: 0;
  overflow-wrap: anywhere;
}
.question-text strong { white-space: pre-wrap; }
.options {
  margin: var(--space-1) 0 0;
  padding: 0;
  list-style: none;
  font-size: var(--text-sm);
  color: var(--color-ink-muted);
}
.options .correct {
  color: var(--color-ink);
  font-weight: var(--weight-semibold);
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.actions button,
.add {
  min-height: var(--touch-min);
}
.icon {
  min-width: 44px;
  padding: 0;
}
.add {
  align-self: flex-start;
}
.form {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
  padding: var(--space-4);
  border-radius: var(--radius-md);
  background: var(--color-surface-alt);
}
.grid {
  display: grid;
  grid-template-columns: minmax(120px, max-content) minmax(0, 1fr);
  align-items: center;
  gap: var(--space-2) var(--space-3);
}
.grid .field-error,
.grid .hint {
  grid-column: 2;
  margin: 0;
  font-size: var(--text-sm);
}
.field-error {
  margin: 0;
  color: var(--color-accent-ink);
}
.form input,
.form select,
.form textarea {
  min-height: var(--touch-min);
  font: inherit;
}
.options-edit {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
  padding: var(--space-3);
  border: 1px solid var(--color-border-soft);
  border-radius: var(--radius-md);
}
.option-row {
  display: grid;
  grid-template-columns: 44px minmax(110px, max-content) minmax(0, 1fr);
  align-items: center;
  gap: var(--space-2);
}
.option-row input[type='radio'] {
  width: 22px;
  height: 22px;
  min-height: 0;
  justify-self: center;
}
.check {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: var(--touch-min);
}
.check input {
  width: 22px;
  height: 22px;
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
@media (max-width: 560px) {
  .grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .grid .field-error,
  .grid .hint {
    grid-column: 1;
  }
  .option-row {
    grid-template-columns: 44px minmax(0, 1fr);
  }
  .option-row input[type='text'] {
    grid-column: 1 / -1;
  }
}
</style>

<style scoped src="../styles/authoring.css"></style>
