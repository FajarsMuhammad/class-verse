<script setup lang="ts">
import ListPagination from './ListPagination.vue'
import { computed, nextTick, onMounted, ref, useId, watch } from 'vue'
import { ApiError, errorMessage } from '@/api/client'
import { levels } from '@/api/modules'
import { lastPage } from '@/api/page'
import { quizApi, shapes, type QuizResult } from '@/api/quizzes'
import type { Level, QuestionInput, Quiz, QuizInput, QuizQuestion } from '@/api/types'
import { useAuthorLeave } from './useAuthorLeave'
import QuizPlayer from './QuizPlayer.vue'
import StatusTabs from './StatusTabs.vue'
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
const statusFilter = ref('all')
const buildingFilter = ref('')
const search = ref('')
const page = ref(1)
const total = ref(0)
const pageSize = 20
const counts = ref({ all: 0, published: 0, draft: 0, empty: 0 })
const statusTabs = computed(() => [
 { key: 'all', label: 'Semua', count: counts.value.all },
 { key: 'published', label: 'Terbit', count: counts.value.published },
 { key: 'draft', label: 'Draf', count: counts.value.draft },
])
// Filters run on the server before paging.
const visible = quizzes
watch([statusFilter, levelFilter, buildingFilter, search], () => { page.value = 1; void load() })
watch(page, () => { void load() })

const unpublishable = computed(() => quizzes.value.filter(q => !q.published && !q.question_count))
// Results: each participant's best finished score (the Rapor rule), paged by the server.
const results = ref<QuizResult[]>([])
const resultsTotal = ref(0)
const resultsPage = ref(1)
const resultsFor = ref<string | null>(null)
async function loadResults() {
  const quiz = resultsFor.value
  if (!quiz) return
  try {
    const r = await quizApi.results(props.orgId, quiz, (resultsPage.value - 1) * pageSize)
    if (resultsFor.value === quiz) { results.value = r.items; resultsTotal.value = r.total }
  } catch (err) { error.value = errorMessage(err) }
}
watch(resultsPage, loadResults)
function showResults(item: Quiz) {
  resultsFor.value = item.id; results.value = []; resultsTotal.value = 0
  if (resultsPage.value === 1) void loadResults(); else resultsPage.value = 1
}
const when = (v: string) => new Date(v).toLocaleString('id-ID', { timeZone: 'Asia/Jakarta', dateStyle: 'medium', timeStyle: 'short' })
const active = computed(() => quizzes.value.find((item) => item.id === selected.value))
const preview = ref(false)
const history=ref<{id:string;revision:number}[]>([])
const formBaseline=ref(''),questionBaseline=ref('')
const conflict=ref(false)
const mayLeave=useAuthorLeave(computed(()=>(!!editing.value && JSON.stringify(form.value)!==formBaseline.value) || (!!questionFor.value && JSON.stringify(question.value)!==questionBaseline.value)))
async function copyLocal(){try{await navigator.clipboard.writeText(JSON.stringify(editing.value?form.value:question.value));message.value='Perubahan lokal disalin.'}catch{error.value='Gagal menyalin. Salin isi form sebelum memuat ulang.'}}
async function reloadDraft(){if(!window.confirm('Buang perubahan lokal dan muat draf server?'))return;await load();const z=active.value;if(z){if(editing.value)edit(z);else editQuestion(z,z.questions?.find(q=>q.id===questionEditing.value))}conflict.value=false}

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
  quizApi.history(props.orgId,item.id).then(r=>history.value=r.items).catch(err=>error.value=errorMessage(err))
  void focusEditor()
}
function back() {
  if(!mayLeave())return
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

let listGeneration = 0
async function load() {
 const generation = ++listGeneration
  try {
    const result = await quizApi.list(props.orgId, { drafts: true, counts: true, building_key: buildingFilter.value, level: levelFilter.value, search: search.value, status: statusFilter.value === 'all' ? '' : statusFilter.value, offset: (page.value - 1) * pageSize, limit: pageSize })
    if (generation !== listGeneration) return
    const last = lastPage(page.value, result)
    if (last !== page.value) { page.value = last; return }
    quizzes.value = result.items
    total.value = result.total ?? result.items.length
    if (result.counts) counts.value = result.counts
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
    if (err instanceof ApiError) {fields.value = err.fields;conflict.value=err.code==='draft_conflict'}
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
    ? { building_key: z.building_key, level: z.level, title: z.title, description: z.description, pass_score: z.pass_score, published: false, expected_version:z.draft_version }
    : { ...blank(), level: levelFilter.value || 'SD', building_key: choices.value[0]?.key ?? '' }
 formBaseline.value=JSON.stringify(form.value);conflict.value=false
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
 if(!window.confirm('Terbitkan revisi kuis baru? Modul yang sudah terbit tetap memakai revisi kuis sebelumnya.'))return
 await run(()=>quizApi.publish(props.orgId,item.id,item.draft_version!), 'Revisi kuis diterbitkan.')
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
 questionBaseline.value=JSON.stringify(question.value);conflict.value=false
}
async function saveQuestion() {
  const qid = questionFor.value!
  const body = { ...question.value, options: [...question.value.options], expected_version:active.value?.draft_version }
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
  await run(() => quizApi.deleteQuestion(props.orgId, z.id, q.id,z.draft_version), 'Soal dihapus.')
}
async function move(z: Quiz, index: number, delta: -1 | 1) {
  const order = (z.questions ?? []).map((q) => q.id)
  const [moved] = order.splice(index, 1)
  order.splice(index + delta, 0, moved!)
  await run(() => quizApi.reorder(props.orgId, z.id, order,z.draft_version), `Urutan ${z.title} disimpan.`)
}

onMounted(async () => {
  await Promise.all([
    load(),
    fetchMap('campus').then((map) => (buildings.value = map.buildings)).catch((err) => (error.value = errorMessage(err))),
  ])
})
</script>

<template>
  <section class="manager authoring quiz-manager studio" :class="{ 'studio-active': selected || editing }" :aria-labelledby="`${id}-title`">
    <header v-if="!selected && !editing" class="head page-head">
      <div>
        <h2 :id="`${id}-title`" class="title">Kuis</h2>
        <p class="sub">Pilihan ganda (4 opsi) atau benar/salah. Server menilai; kunci dan pembahasan terlihat setelah kuis selesai.
          Siswa boleh mengulang, rapor memakai nilai tertinggi.</p>
      </div>
      <button v-if="!editing" type="button" class="btn-primary" :disabled="!choices.length" @click="edit()">+ Kuis baru</button>
    </header>
    <div v-if="!selected && !editing && counts.empty" class="banner warn" role="note">
      <strong>{{ counts.empty }} kuis belum punya soal</strong>
      <span><template v-if="unpublishable.length">Di halaman ini: {{ unpublishable.map((q) => `“${q.title}”`).join(', ') }}. </template>Kuis butuh minimal 1 soal sebelum diterbitkan.</span>
    </div>
    <p v-if="message" class="notice" role="status">{{ message }}</p>
    <p v-if="error" class="alert" role="alert">{{ error }} {{ !editing ? fields.questions : '' }}</p>

    <div v-if="conflict" class="actions"><button type="button" class="btn-secondary" @click="copyLocal">Salin perubahan lokal</button><button type="button" class="btn-secondary" @click="reloadDraft">Muat versi server</button></div>
    <details v-if="active && history.length"><summary>Riwayat revisi terbit (hanya baca)</summary><ol><li v-for="r in history" :key="r.id">Revisi {{ r.revision }}</li></ol></details>
    <header v-if="selected || editing" class="editor-head">
      <button type="button" class="btn-secondary" data-action="back" :disabled="busy" @click="back">Kembali ke daftar</button>
      <h3 ref="editorHeading" class="editor-title" tabindex="-1">{{ active?.title || 'Kuis baru' }}</h3>
      <span v-if="active" class="chip">{{ active.published ? 'Terbit' : 'Draf' }}</span>
    </header>
    <div v-if="active && !editing && !questionFor" class="editor-actions actions">
      <button type="button" class="btn-secondary" :aria-pressed="preview" @click="preview = !preview">{{ preview ? 'Kembali menyusun' : 'Pratinjau isi' }}</button>
      <button type="button" class="btn-secondary" :disabled="busy" @click="edit(active)">Pengaturan</button>
      <button type="button" class="btn-primary" :disabled="busy || (!active.published && !active.question_count)" @click="setPublished(active)">{{ active.published ? 'Terbitkan revisi baru' : 'Terbitkan' }}</button>
    </div>
    <p v-if="active && !active.published && !active.question_count" class="hint">Tambahkan minimal satu soal sebelum menerbitkan.</p>
    <div v-if="!selected && !editing" class="catalog">
      <div class="toolbar">
        <StatusTabs v-model="statusFilter" :items="statusTabs" label="Status kuis" :controls="`${id}-table`" />
        <div class="filters">
          <label :for="`${id}-building`">Gedung</label>
          <select :id="`${id}-building`" v-model="buildingFilter"><option value="">Semua gedung</option><option v-for="b in choices" :key="b.key" :value="b.key">{{ b.name }}</option></select>
          <label :for="`${id}-filter`">Jenjang</label>
          <select :id="`${id}-filter`" v-model="levelFilter" aria-label="Filter jenjang">
            <option value="">Semua jenjang</option>
            <option v-for="level in levels" :key="level" :value="level">{{ level }}</option>
          </select>
        </div>
      </div>
      <label :for="`${id}-search`" class="sr">Cari kuis</label>
      <input :id="`${id}-search`" v-model="search" type="search" class="search" placeholder="Cari kuis…" />
      <p v-if="!visible.length" class="empty">{{ counts.all ? 'Tidak ada kuis yang cocok dengan filter ini.' : 'Belum ada kuis. Buat kuis pertama.' }}</p>
      <div v-else class="table-wrap">
        <table :id="`${id}-table`" class="data-table">
          <thead><tr><th scope="col">Kuis</th><th scope="col">Soal</th><th scope="col">Nilai lulus</th><th scope="col">Modul</th><th scope="col">Status</th><th scope="col"><span class="sr">Aksi</span></th></tr></thead>
          <tbody>
            <tr v-for="item in visible" :key="item.id" class="catalog-row">
              <th scope="row"><strong>{{ item.title }}</strong><small>{{ buildingName(item.building_key) }} · {{ item.level }}</small></th>
              <td :class="{ danger: !item.question_count }">{{ item.question_count }} soal</td>
              <td>{{ item.pass_score }}</td>
              <td><small>{{ item.module_titles?.length ? item.module_titles.join(', ') : 'Belum dipakai modul' }}</small></td>
              <td><span :class="['chip', item.published ? 'ok' : 'draft']">{{ item.published ? 'Terbit' : 'Draf' }}</span></td>
              <td class="row-actions">
                <button type="button" :class="item.published || item.question_count ? 'btn-secondary' : 'btn-primary'" :aria-label="`${item.published ? 'Buka' : item.question_count ? 'Edit soal' : 'Tambah soal'} ${item.title}`" @click="openItem(item)">{{ item.published ? 'Buka' : item.question_count ? 'Edit soal' : '+ Tambah soal' }}</button>
                <button v-if="item.published" type="button" class="btn-secondary" :aria-label="`Lihat hasil ${item.title}`" @click="showResults(item)">Lihat hasil</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="hint">Kuis dengan 1–2 soal kurang akurat untuk rapor. Disarankan minimal 5 soal.</p>
    </div>
    <ListPagination v-if="!selected && !editing" v-model="page" :total="total" :size="pageSize" label="Halaman kuis" />
    <section v-if="resultsFor && !selected && !editing" class="results" :aria-labelledby="`${id}-results`">
      <header class="editor-head">
        <h3 :id="`${id}-results`">Hasil: {{ quizzes.find(q => q.id === resultsFor)?.title ?? 'kuis' }}</h3>
        <button type="button" class="btn-secondary" @click="resultsFor = null">Tutup hasil</button>
      </header>
      <p class="hint">Nilai tertinggi tiap siswa dari percobaan yang selesai, sama dengan Rapor.</p>
      <p v-if="!results.length" class="empty">Belum ada siswa yang menyelesaikan kuis ini.</p>
      <div v-else class="table-wrap">
        <table class="data-table">
          <thead><tr><th scope="col">Siswa</th><th scope="col">Nilai tertinggi</th><th scope="col">Percobaan</th><th scope="col">Terakhir selesai</th></tr></thead>
          <tbody><tr v-for="r in results" :key="r.user_id"><th scope="row">{{ r.display_name }}</th><td>{{ r.best_score }}</td><td>{{ r.attempts }}</td><td>{{ when(r.last_finished_at) }}</td></tr></tbody>
        </table>
      </div>
      <ListPagination v-model="resultsPage" :total="resultsTotal" :size="pageSize" label="Halaman hasil kuis" />
    </section>
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
        <button type="button" class="btn-secondary" :disabled="busy" @click="mayLeave() && (editing = null)">Batal</button>
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
      <div class="studio-workspace" :class="{ 'is-editing': !!questionFor }">
      <nav v-if="!preview" class="studio-outline" aria-label="Daftar soal">
        <h4>Soal</h4>
        <button v-for="(q, i) in z.questions ?? []" :key="q.id" type="button" :aria-current="questionEditing === q.id && questionFor ? 'step' : undefined" :disabled="busy" @click="mayLeave() && editQuestion(z, q)"><span class="outline-number">{{ i + 1 }}</span><span><small>{{ q.options.length === 2 ? 'Benar / Salah' : 'Pilihan ganda' }}</small>{{ q.prompt }}</span></button>
        <button type="button" class="outline-add" :disabled="busy" @click="mayLeave() && editQuestion(z)">+ Tambah soal</button>
      </nav>
      <div class="studio-content">
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
        <label :for="`${id}-type`">Jenis soal</label>
        <select :id="`${id}-type`" :value="question.options.length" @change="question={...question,answer:0,options:($event.target as HTMLSelectElement).value==='2'?['Benar','Salah']:['','','','']}">
          <option value="4">Pilihan tunggal (4 opsi)</option><option value="2">Benar / salah</option>
        </select>
        <fieldset class="options-edit">
          <legend>Pilihan (tandai jawaban benar)</legend>
          <div v-for="(_, oi) in question.options" :key="oi" :class="['option-row', `answer-${oi}`, { 'is-correct': question.answer === oi }]">
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
          <button type="button" class="btn-secondary" :disabled="busy" @click="mayLeave() && (questionFor = null)">Batal</button>
        </div>
      </form>
      <button v-else-if="!preview" type="button" class="btn-primary add" :disabled="busy" @click="editQuestion(z)">+ Soal<span class="sr"> di {{ z.title }}</span></button>
      </div>
      <aside v-if="!preview" class="studio-settings" aria-label="Aturan kuis">
        <h4>Mode kuis</h4><div class="studio-mode"><strong>Latihan</strong><p>Boleh diulang. Pembahasan muncul setelah kuis selesai.</p></div>
        <h4>Aturan</h4><dl><dt>Nilai lulus</dt><dd>{{ z.pass_score }}</dd><dt>Percobaan</dt><dd>Tak terbatas</dd><dt>Penilaian</dt><dd>Bobot soal sama</dd></dl>
        <p class="hint">Rapor memakai nilai tertinggi. Pratinjau tidak menyimpan hasil belajar.</p>
      </aside>
      </div>
    </article>
    <QuizPlayer v-if="preview && active" :org-id="orgId" :quiz-id="active.id" :title="active.title" :preview-questions="active.questions ?? []" :pass-score="active.pass_score" @close="preview=false" />
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
