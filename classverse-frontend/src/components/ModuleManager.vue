<script setup lang="ts">
import { computed, nextTick, onMounted, ref, useId, watch } from 'vue'
import type { RoomHandle } from '@/realtime/useRoom'
import { ApiError, errorMessage } from '@/api/client'
import { fileApi } from '@/api/files'
import { kindLabel, levels, moduleApi } from '@/api/modules'
import { quizApi } from '@/api/quizzes'
import type { FileItem, Level, Module, ModuleInput, ModuleStep, Quiz, StepInput } from '@/api/types'
import { fetchMap, type Building } from '@/realtime/mapdata'

// Kelola → Modul (M1-01): staff author paths per building and level. Steps are archived, never deleted.
// buildingKey limits the manager to one building (Perpustakaan tools open it for `perpus` only).
const props = defineProps<{ orgId: string; buildingKey?: string; room?: RoomHandle }>()
const emit = defineEmits<{ pinned: [] }>()

const id = useId()
const modules = ref<Module[]>([])
const buildings = ref<Building[]>([])
const files = ref<FileItem[]>([])
const quizzes = ref<Quiz[]>([]) // published quizzes of the building whose step form is open
const editing = ref<string | null>(null) // module id or 'new'
const form = ref<ModuleInput>(blank())
const stepFor = ref<string | null>(null) // module id whose step form is open
const stepEditing = ref<string | null>(null) // step id or 'new'
const step = ref<StepInput>(blankStep())
const fields = ref<Record<string, string>>({})
const busy = ref(false)
const message = ref('')
const error = ref('')
const levelFilter = ref<Level | ''>('')
const selected = ref<string | null>(null)
const visible = computed(() => modules.value.filter((item) => !levelFilter.value || item.level === levelFilter.value))
const active = computed(() => modules.value.find((item) => item.id === selected.value))
const preview = ref(false)
const pinChoosing = ref(false)
const pinStep = ref('')
const pinRoom = computed(() => props.room?.current.value)
const canPin = computed(() => !!props.room?.pinLearning && props.room.status.value === 'open' &&
  !props.room.traveling?.value && !props.room.kicked?.value && !!pinRoom.value && pinRoom.value.map_key !== 'campus')
watch(() => pinRoom.value?.room_id, () => { pinChoosing.value = false; pinStep.value = '' })
watch(selected, () => { pinChoosing.value = false; pinStep.value = '' })
function pinModule() {
  const item = active.value
  if (!item?.published || !item.steps.length || !canPin.value) return
  if (pinStep.value && !item.steps.some((step) => step.id === pinStep.value)) return
  props.room?.pinLearning?.(item.id, pinStep.value || undefined)
  pinChoosing.value = false
  emit('pinned')
}
const editorHeading = ref<HTMLElement | null>(null)
async function focusEditor() {
  await nextTick()
  editorHeading.value?.focus()
}
function openItem(item: Module) {
  selected.value = item.id
  preview.value = false
  editing.value = null
  stepFor.value = null
  fields.value = {}
  error.value = ''
  void focusEditor()
}
function back() {
  if ((editing.value || stepFor.value) && !window.confirm('Tutup form tanpa menyimpan perubahan?')) return
  selected.value = null
  editing.value = null
  stepFor.value = null
  preview.value = false
}


function blank(): ModuleInput {
  return { building_key: '', level: 'SD', title: '', description: '', published: false }
}
function blankStep(): StepInput {
  return { kind: 'materi', title: '', body: '', file_id: null, video_id: '', quiz_id: null, minutes: 5 }
}
// Ruang Guru is staff-only and has no learning paths (the server rejects it too).
const choices = computed(() => buildings.value.filter((b) => !b.staffOnly && (!props.buildingKey || b.key === props.buildingKey)))
const buildingName = (key: string) => buildings.value.find((b) => b.key === key)?.name ?? key

async function load() {
  try {
    modules.value = (await moduleApi.list(props.orgId, { drafts: true, building_key: props.buildingKey })).items
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

function edit(m?: Module) {
  selected.value = m?.id ?? null
  preview.value = false
  stepFor.value = null
  editing.value = m?.id ?? 'new'
  fields.value = {}
  form.value = m
    ? { building_key: m.building_key, level: m.level, title: m.title, description: m.description, published: m.published }
    : { ...blank(), level: levelFilter.value || 'SD', building_key: choices.value[0]?.key ?? '' }
}
async function save() {
  const body = form.value
  const ok = await run(
    async () => {
      const saved = editing.value === 'new'
        ? await moduleApi.create(props.orgId, { ...body, published: false })
        : await moduleApi.update(props.orgId, editing.value!, body)
      selected.value = saved.id
    },
    `${body.title.trim()} ${body.published ? 'terbit' : 'disimpan sebagai draf'}.`,
  )
  if (ok) {
    editing.value = null
    const item = active.value
    if (item && !item.steps.length) editStep(item)
    void focusEditor()
  }
}
async function setPublished(item: Module) {
  const { building_key, level, title, description } = item
  await run(() => moduleApi.update(props.orgId, item.id, {
    building_key, level, title, description, published: !item.published,
  }), item.published ? 'Kembali menjadi draf.' : 'Modul diterbitkan.')
}
async function archive(m: Module) {
  if (!window.confirm(`Arsipkan modul ${m.title}? Peserta tidak bisa membukanya lagi; file tetap ada.`)) return
  if (await run(() => moduleApi.archive(props.orgId, m.id), `${m.title} diarsipkan.`)) selected.value = null
}

function editStep(m: Module, s?: ModuleStep) {
  editing.value = null
  stepFor.value = m.id
  stepEditing.value = s?.id ?? 'new'
  fields.value = {}
  step.value = s
    ? { kind: s.kind, title: s.title, body: s.body, file_id: s.file_id, video_id: s.video_id ?? '', quiz_id: s.quiz_id, minutes: s.minutes }
    : blankStep()
  if (!files.value.length) {
    fileApi.list(props.orgId).then((r) => (files.value = r.items)).catch((err) => { error.value = errorMessage(err) })
  }
  // Only published quizzes of this building can be linked (the server checks too).
  quizApi.list(props.orgId, { building_key: m.building_key }).then((r) => (quizzes.value = r.items)).catch((err) => { error.value = errorMessage(err) })
}
async function saveStep() {
  const mid = stepFor.value!
  const body = {
    ...step.value,
    file_id: step.value.kind === 'berkas' ? step.value.file_id : null,
    quiz_id: step.value.kind === 'kuis' ? step.value.quiz_id : null,
  }
  const ok = await run(
    () => (stepEditing.value === 'new'
      ? moduleApi.addStep(props.orgId, mid, body)
      : moduleApi.updateStep(props.orgId, mid, stepEditing.value!, body)),
    `Langkah ${body.title.trim()} disimpan.`,
  )
  if (ok) stepFor.value = null
}
async function archiveStep(m: Module, s: ModuleStep) {
  if (!window.confirm(`Arsipkan langkah ${s.title}?`)) return
  await run(() => moduleApi.archiveStep(props.orgId, m.id, s.id), `Langkah ${s.title} diarsipkan.`)
}
async function move(m: Module, index: number, delta: -1 | 1) {
  const order = m.steps.map((s) => s.id)
  const [moved] = order.splice(index, 1)
  order.splice(index + delta, 0, moved!)
  await run(() => moduleApi.reorder(props.orgId, m.id, order), `Urutan ${m.title} disimpan.`)
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
      <h2 :id="`${id}-title`" class="title">Modul belajar</h2>
      <button v-if="!editing" type="button" class="btn-primary" :disabled="!choices.length" @click="edit()">+ Modul baru</button>
    </header>
    <p v-if="!selected && !editing" class="sub">Jalur belajar per gedung dan jenjang. Draf hanya terlihat pengajar; peserta membuka modul terbit dari detail gedung.</p>
    <p v-if="message" class="notice" role="status">{{ message }}</p>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>

    <header v-if="selected || editing" class="editor-head">
      <button type="button" class="btn-secondary" data-action="back" :disabled="busy" @click="back">Kembali ke daftar</button>
      <h3 ref="editorHeading" class="editor-title" tabindex="-1">{{ active?.title || 'Modul baru' }}</h3>
      <span v-if="active" class="chip">{{ active.published ? 'Terbit' : 'Draf' }}</span>
    </header>
    <div v-if="active && !editing && !stepFor" class="editor-actions actions">
      <button type="button" class="btn-secondary" :aria-pressed="preview" @click="preview = !preview">{{ preview ? 'Kembali menyusun' : 'Pratinjau isi' }}</button>
      <button type="button" class="btn-secondary" :disabled="busy" @click="edit(active)">Pengaturan</button>
      <button type="button" class="btn-primary" :disabled="busy || (!active.published && !active.steps.length)" @click="setPublished(active)">{{ active.published ? 'Jadikan draf' : 'Terbitkan' }}</button>
    </div>
    <template v-if="active && room && !editing && !stepFor">
      <p v-if="pinRoom?.map_key === 'campus'" class="hint">Masuk ruang untuk menyematkan modul.</p>
      <template v-else-if="active.published && active.steps.length">
        <button v-if="!pinChoosing" type="button" class="btn-secondary pin-action" data-action="pin" :disabled="!canPin" @click="pinChoosing = true">
          Sematkan ke {{ pinRoom?.room_name || 'ruang ini' }}
        </button>
        <form v-else class="pin-form" @submit.prevent="pinModule">
          <strong>Sematkan ke {{ pinRoom?.room_name || 'ruang ini' }}</strong>
          <label :for="`${id}-pin-step`">Mulai dari</label>
          <select :id="`${id}-pin-step`" v-model="pinStep">
            <option value="">Seluruh modul</option>
            <option v-for="s in active.steps" :key="s.id" :value="s.id">{{ s.title }}</option>
          </select>
          <p class="hint">Peserta membuka materi di layar masing-masing; layar mereka tidak berpindah otomatis.</p>
          <div class="actions">
            <button type="submit" class="btn-primary" :disabled="!canPin">Sematkan</button>
            <button type="button" class="btn-secondary" @click="pinChoosing = false">Batal</button>
          </div>
        </form>
      </template>
      <p v-else class="hint">Terbitkan modul berisi materi sebelum menyematkan.</p>
    </template>
    <p v-if="active && !active.published && !active.steps.length" class="hint">Tambahkan minimal satu materi sebelum menerbitkan.</p>
    <div v-if="!selected && !editing" class="catalog">
      <label :for="`${id}-filter`">Jenjang</label>
      <select :id="`${id}-filter`" v-model="levelFilter" aria-label="Filter jenjang">
        <option value="">Semua jenjang</option>
        <option v-for="level in levels" :key="level" :value="level">{{ level }}</option>
      </select>
      <p v-if="!visible.length" class="hint">Belum ada modul untuk jenjang ini.</p>
      <ul class="catalog-list">
        <li v-for="item in visible" :key="item.id" class="catalog-row">
          <div><strong>{{ item.title }}</strong>
            <p class="meta">{{ buildingName(item.building_key) }} · {{ item.level }} · {{ item.steps.length }} langkah</p>
          </div>
          <span class="chip">{{ item.published ? 'Terbit' : 'Draf' }}</span>
          <button type="button" class="btn-secondary" :aria-label="`Buka ${item.title}`" @click="openItem(item)">Buka</button>
        </li>
      </ul>
    </div>
    <form v-if="editing" class="form" novalidate @submit.prevent="save">
      <h3 class="form-title">{{ editing === 'new' ? 'Modul baru' : 'Ubah modul' }}</h3>
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
        <input :id="`${id}-t`" v-model="form.title" maxlength="120" :aria-invalid="!!fields.title"
          :aria-describedby="fields.title ? `${id}-te` : undefined" />
        <p v-if="fields.title" :id="`${id}-te`" class="field-error">{{ fields.title }}</p>
        <label :for="`${id}-d`">Deskripsi</label>
        <textarea :id="`${id}-d`" v-model="form.description" maxlength="500" rows="2" />
      </div>
      <p v-if="editing === 'new'" class="hint">Disimpan sebagai draf. Tambahkan isi, lalu terbitkan.</p>
      <div class="actions">
        <button type="submit" class="btn-primary" :disabled="busy">Simpan</button>
        <button type="button" class="btn-secondary" :disabled="busy" @click="editing = null">Batal</button>
      </div>
    </form>

    <p v-if="!selected && !editing && !modules.length" class="sub">Belum ada modul. Buat modul pertama.</p>
    <article v-for="m in active && !editing ? [active] : []" :key="m.id" class="module" :aria-labelledby="`${id}-${m.id}`">
      <div class="module-head">
        <div>
          <h3 :id="`${id}-${m.id}`">{{ m.title }}</h3>
          <p class="meta">
            {{ buildingName(m.building_key) }} · {{ m.level }} · {{ m.steps.length }} langkah
            <span v-if="!m.published" class="chip">Draf</span>
          </p>
        </div>
        <div v-if="!preview" class="actions">
          <button type="button" class="btn-secondary" :disabled="busy" @click="archive(m)">Arsipkan<span class="sr"> {{ m.title }}</span></button>
        </div>
      </div>
      <ol v-if="!stepFor" class="steps">
        <li v-for="(s, i) in m.steps" :key="s.id">
          <span class="step-title"><strong>{{ i + 1 }}. {{ s.title }}</strong> <small>{{ kindLabel[s.kind] }} · {{ s.minutes }} mnt</small><p v-if="preview" class="preview-body">{{ s.body }}</p></span>
          <span v-if="!preview" class="actions">
            <button type="button" class="btn-secondary icon" :disabled="busy || i === 0" :aria-label="`Naikkan ${s.title}`" @click="move(m, i, -1)">↑</button>
            <button type="button" class="btn-secondary icon" :disabled="busy || i === m.steps.length - 1" :aria-label="`Turunkan ${s.title}`" @click="move(m, i, 1)">↓</button>
            <button type="button" class="btn-secondary" :disabled="busy" @click="editStep(m, s)">Ubah<span class="sr"> langkah {{ s.title }}</span></button>
            <button type="button" class="btn-secondary" :disabled="busy" @click="archiveStep(m, s)">Arsipkan<span class="sr"> langkah {{ s.title }}</span></button>
          </span>
        </li>
      </ol>
      <form v-if="stepFor === m.id" class="form" novalidate @submit.prevent="saveStep">
        <h4 class="form-title">{{ stepEditing === 'new' ? 'Langkah baru' : 'Ubah langkah' }}</h4>
        <div class="grid">
          <label :for="`${id}-sk`">Jenis</label>
          <select :id="`${id}-sk`" v-model="step.kind">
            <option v-for="(label, key) in kindLabel" :key="key" :value="key">{{ label }}</option>
          </select>
          <label :for="`${id}-st`">Judul</label>
          <input :id="`${id}-st`" v-model="step.title" maxlength="120" :aria-invalid="!!fields.title" />
          <p v-if="fields.title" class="field-error">{{ fields.title }}</p>
          <template v-if="step.kind === 'berkas'">
            <label :for="`${id}-sf`">File</label>
            <select :id="`${id}-sf`" v-model="step.file_id" :aria-invalid="!!fields.file_id">
              <option :value="null" disabled>Pilih file</option>
              <option v-for="f in files" :key="f.id" :value="f.id">{{ f.name }}</option>
            </select>
            <p :class="fields.file_id ? 'field-error' : 'hint'">
              {{ fields.file_id || (files.length ? 'PDF atau audio dari File belajar.' : 'Belum ada file. Unggah dulu di File belajar.') }}
            </p>
          </template>
          <template v-if="step.kind === 'kuis'">
            <label :for="`${id}-sq`">Kuis</label>
            <select :id="`${id}-sq`" v-model="step.quiz_id" aria-label="Kuis" :aria-invalid="!!fields.quiz_id">
              <option :value="null" disabled>Pilih kuis</option>
              <option v-for="z in quizzes" :key="z.id" :value="z.id">{{ z.title }} ({{ z.level }}, {{ z.question_count }} soal)</option>
            </select>
            <p :class="fields.quiz_id ? 'field-error' : 'hint'">
              {{ fields.quiz_id || (quizzes.length ? 'Selesai otomatis saat peserta lulus kuis.' : 'Belum ada kuis terbit di gedung ini. Buat di bagian Kuis.') }}
            </p>
          </template>
          <template v-if="step.kind === 'video'">
            <label :for="`${id}-sv`">Tautan YouTube</label>
            <input :id="`${id}-sv`" v-model="step.video_id" inputmode="url" placeholder="https://youtu.be/…"
              :aria-invalid="!!fields.video_id" />
            <p :class="fields.video_id ? 'field-error' : 'hint'">{{ fields.video_id || 'Diputar di dalam aplikasi lewat youtube-nocookie.com.' }}</p>
          </template>
          <label :for="`${id}-sb`">{{ step.kind === 'materi' ? 'Isi materi' : 'Catatan (opsional)' }}</label>
          <textarea :id="`${id}-sb`" v-model="step.body" maxlength="5000" :rows="step.kind === 'materi' ? 6 : 2"
            :aria-invalid="!!fields.body" />
          <p v-if="fields.body" class="field-error">{{ fields.body }}</p>
          <label :for="`${id}-sm`">Durasi (menit)</label>
          <input :id="`${id}-sm`" v-model.number="step.minutes" type="number" min="1" max="240" inputmode="numeric"
            :aria-invalid="!!fields.minutes" />
          <p v-if="fields.minutes" class="field-error">{{ fields.minutes }}</p>
        </div>
        <p v-if="fields.steps" class="field-error">{{ fields.steps }}</p>
        <div class="actions">
          <button type="submit" class="btn-primary" :disabled="busy">Simpan langkah</button>
          <button type="button" class="btn-secondary" :disabled="busy" @click="stepFor = null">Batal</button>
        </div>
      </form>
      <button v-else-if="!preview" type="button" class="btn-primary add" :disabled="busy" @click="editStep(m)">+ Langkah<span class="sr"> di {{ m.title }}</span></button>
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
.module-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
}
.title,
.sub,
.form-title,
.module h3,
.meta {
  margin: 0;
}
.title {
  font-family: var(--font-display);
  font-size: var(--text-xl);
}
.sub,
.meta,
.hint,
small {
  color: var(--color-ink-muted);
}
.module {
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
.steps {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}
.steps li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  background: var(--color-surface-alt);
}
.step-title {
  min-width: 0;
  overflow-wrap: anywhere;
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
  color: var(--color-accent-ink);
}
.grid input,
.grid select,
.grid textarea {
  min-height: var(--touch-min);
  font: inherit;
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
}
</style>

<style scoped src="../styles/authoring.css"></style>
