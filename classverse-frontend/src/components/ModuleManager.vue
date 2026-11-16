<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'
import { ApiError, errorMessage } from '@/api/client'
import { fileApi } from '@/api/files'
import { kindLabel, levels, moduleApi } from '@/api/modules'
import { quizApi } from '@/api/quizzes'
import type { FileItem, Module, ModuleInput, ModuleStep, Quiz, StepInput } from '@/api/types'
import { fetchMap, type Building } from '@/realtime/mapdata'

// Kelola → Modul (M1-01): staff author paths per building and level. Steps are archived, never deleted.
// buildingKey limits the manager to one building (Perpustakaan tools open it for `perpus` only).
const props = defineProps<{ orgId: string; buildingKey?: string }>()

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
  stepFor.value = null
  editing.value = m?.id ?? 'new'
  fields.value = {}
  form.value = m
    ? { building_key: m.building_key, level: m.level, title: m.title, description: m.description, published: m.published }
    : { ...blank(), building_key: choices.value[0]?.key ?? '' }
}
async function save() {
  const body = form.value
  const ok = await run(
    () => (editing.value === 'new' ? moduleApi.create(props.orgId, body) : moduleApi.update(props.orgId, editing.value!, body)),
    `${body.title.trim()} ${body.published ? 'terbit' : 'disimpan sebagai draf'}.`,
  )
  if (ok) editing.value = null
}
async function archive(m: Module) {
  if (!window.confirm(`Arsipkan modul ${m.title}? Peserta tidak bisa membukanya lagi; file tetap ada.`)) return
  await run(() => moduleApi.archive(props.orgId, m.id), `${m.title} diarsipkan.`)
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
    fileApi.list(props.orgId).then((r) => (files.value = r.items)).catch(() => (files.value = []))
  }
  // Only published quizzes of this building can be linked (the server checks too).
  quizApi.list(props.orgId, { building_key: m.building_key }).then((r) => (quizzes.value = r.items)).catch(() => (quizzes.value = []))
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
  <section class="manager" :aria-labelledby="`${id}-title`">
    <header class="head">
      <h2 :id="`${id}-title`" class="title">Modul belajar</h2>
      <button v-if="!editing" type="button" class="btn-primary" :disabled="!choices.length" @click="edit()">+ Modul baru</button>
    </header>
    <p class="sub">Jalur belajar per gedung dan jenjang. Draf hanya terlihat pengajar; peserta membuka modul terbit dari detail gedung.</p>
    <p v-if="message" class="notice" role="status">{{ message }}</p>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>

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
      <label class="check"><input v-model="form.published" type="checkbox" /> Terbitkan untuk peserta</label>
      <div class="actions">
        <button type="submit" class="btn-primary" :disabled="busy">Simpan</button>
        <button type="button" class="btn-secondary" :disabled="busy" @click="editing = null">Batal</button>
      </div>
    </form>

    <p v-if="!modules.length" class="sub">Belum ada modul. Buat modul pertama.</p>
    <article v-for="m in modules" :key="m.id" class="module" :aria-labelledby="`${id}-${m.id}`">
      <div class="module-head">
        <div>
          <h3 :id="`${id}-${m.id}`">{{ m.title }}</h3>
          <p class="meta">
            {{ buildingName(m.building_key) }} · {{ m.level }} · {{ m.steps.length }} langkah
            <span v-if="!m.published" class="chip">Draf</span>
          </p>
        </div>
        <div class="actions">
          <button type="button" class="btn-secondary" :disabled="busy" @click="edit(m)">Ubah<span class="sr"> {{ m.title }}</span></button>
          <button type="button" class="btn-secondary" :disabled="busy" @click="archive(m)">Arsipkan<span class="sr"> {{ m.title }}</span></button>
        </div>
      </div>
      <ol class="steps">
        <li v-for="(s, i) in m.steps" :key="s.id">
          <span class="step-title"><strong>{{ i + 1 }}. {{ s.title }}</strong> <small>{{ kindLabel[s.kind] }} · {{ s.minutes }} mnt</small></span>
          <span class="actions">
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
      <button v-else type="button" class="btn-secondary add" :disabled="busy" @click="editStep(m)">+ Langkah<span class="sr"> di {{ m.title }}</span></button>
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
  min-height: 44px;
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
  min-height: 44px;
  font: inherit;
}
.check {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 44px;
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
