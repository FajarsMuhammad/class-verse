<script setup lang="ts">
import PagedSelect from './PagedSelect.vue'
import ListPagination from './ListPagination.vue'
import { computed, nextTick, onMounted, ref, useId, watch } from 'vue'
import type { RoomHandle } from '@/realtime/useRoom'
import { ApiError, errorMessage } from '@/api/client'
import { fileApi } from '@/api/files'
import { hasMore, lastPage } from '@/api/page'
import { kindLabel, levels, moduleApi } from '@/api/modules'
import { quizApi } from '@/api/quizzes'
import type { Level, Module, ModuleInput, ModuleStep, StepInput } from '@/api/types'
import RichEditor from './RichEditor.vue'
import StatusTabs from './StatusTabs.vue'
import RichContent from './RichContent.vue'
import { templateRich } from './richtext'
import { useAuthorLeave } from './useAuthorLeave'
import { useDraftSave } from './useDraftSave'
import { fetchMap, type Building } from '@/realtime/mapdata'

// Kelola → Modul (M1-01): staff author paths per building and level. Steps are archived, never deleted.
// buildingKey limits the manager to one building (Perpustakaan tools open it for `perpus` only).
const props = defineProps<{ orgId: string; buildingKey?: string; room?: RoomHandle }>()
const emit = defineEmits<{ pinned: []; assign: [moduleId: string] }>()

const id = useId()
const modules = ref<Module[]>([])
const buildings = ref<Building[]>([])
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
const statusFilter = ref('all')
const buildingFilter = ref('')
const search = ref('')
const page = ref(1)
const total = ref(0)
const pageSize = 20
const counts = ref({ all: 0, published: 0, draft: 0, archived: 0 })
const statusTabs = computed(() => [
 { key: 'all', label: 'Semua', count: counts.value.all },
 { key: 'published', label: 'Terbit', count: counts.value.published },
 { key: 'draft', label: 'Draf', count: counts.value.draft },
 { key: 'archived', label: 'Arsip', count: counts.value.archived },
])
// Server already filters, searches, and pages; the table shows the page as returned.
const visible = modules
const filtered = computed(() => !!(levelFilter.value || buildingFilter.value || search.value.trim() || statusFilter.value !== 'all'))
watch([statusFilter, levelFilter, buildingFilter, search], () => { page.value = 1; void load() })
watch(page, () => { void load() })

const active = computed(() => modules.value.find((item) => item.id === selected.value))
const preview = ref(false)
const starter = ref<'empty' | 'reading' | 'practice'>('empty')
const history = ref<{id: string; revision: number}[]>([])
const stepVersion = ref(0)
const stepSave = useDraftSave(step, async () => {
 const mid=stepFor.value
 if (!mid) return
 const body={...step.value,expected_version:stepVersion.value}
 let saved: ModuleStep
 try { saved=stepEditing.value==='new' ? await moduleApi.addStep(props.orgId,mid,body) : await moduleApi.updateStep(props.orgId,mid,stepEditing.value!,body) } catch(err){if(err instanceof ApiError)fields.value=err.fields;throw err}
 stepEditing.value=saved.id
 stepVersion.value++
 await load()
})
async function reloadDraft() {
 if (!window.confirm('Buang perubahan lokal dan muat draf server?')) return
 stepSave.discard(); await load()
 const m=active.value,s=m?.steps.find(s=>s.id===stepEditing.value)
 if(m) editStep(m,s)
}
async function copyDraft() {
 try {await navigator.clipboard.writeText(JSON.stringify(step.value));message.value='Perubahan lokal disalin.'}
 catch {error.value='Gagal menyalin. Pilih dan salin isi editor sebelum memuat ulang.'}
}
const formBaseline=ref('')
const formDirty=computed(()=>!!editing.value && JSON.stringify(form.value)!==formBaseline.value)
const mayLeave=useAuthorLeave(computed(()=>stepSave.dirty.value || formDirty.value))
const formConflict=ref(false)
async function copyLocal(){try{await navigator.clipboard.writeText(JSON.stringify(editing.value?form.value:step.value));message.value='Perubahan lokal disalin.'}catch{error.value='Gagal menyalin; salin isi form sebelum memuat versi server.'}}
async function reloadForm(){if(!window.confirm('Buang perubahan lokal dan muat draf server?'))return;await load();if(active.value)edit(active.value);formConflict.value=false}

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
  stepSave.discard()
  selected.value = item.id
  preview.value = false
  editing.value = null
  stepFor.value = null
  fields.value = {}
  error.value = ''
  moduleApi.history(props.orgId,item.id).then(r=>history.value=r.items).catch(err=>error.value=errorMessage(err))
  void focusEditor()
}
function back() {
  if (!mayLeave()) return
  stepSave.discard()
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

let listGeneration = 0
async function load() {
 const generation = ++listGeneration
  try {
    const result = await moduleApi.list(props.orgId, { drafts: true, counts: true, archived: statusFilter.value === 'archived', building_key: props.buildingKey || buildingFilter.value, level: levelFilter.value, search: search.value, status: ['draft', 'published'].includes(statusFilter.value) ? statusFilter.value : '', offset: (page.value - 1) * pageSize, limit: pageSize })
    if (generation !== listGeneration) return
    // Removing the last item of the last page: the server cannot total an offset past the end, so step back.
    const last = lastPage(page.value, result)
    if (last !== page.value) { page.value = last; return }
    modules.value = result.items
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
    if (err instanceof ApiError) {fields.value = err.fields;formConflict.value=err.code==='draft_conflict'}
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
  starter.value = m ? 'empty' : 'reading'
  fields.value = {}
  form.value = m
    ? { building_key: m.building_key, level: m.level, title: m.title, description: m.description, published: false, expected_version:m.draft_version }
    : { ...blank(), level: levelFilter.value || 'SD', building_key: choices.value[0]?.key ?? '' }
  formBaseline.value=JSON.stringify(form.value);formConflict.value=false
}
async function save() {
  const body = form.value
  const ok = await run(
    async () => {
      const saved = editing.value === 'new'
        ? await moduleApi.create(props.orgId, { ...body, published: false })
        : await moduleApi.update(props.orgId, editing.value!, body)
      selected.value = saved.id
      if (editing.value === 'new' && starter.value !== 'empty') {
        editing.value = saved.id
        form.value = { ...body, expected_version: saved.draft_version }
        formBaseline.value = JSON.stringify(form.value)
        const titles = starter.value === 'reading'
          ? ['Pendahuluan', 'Materi', 'Contoh', 'Rangkuman']
          : ['Tujuan', 'Alat dan bahan', 'Langkah', 'Hasil', 'Refleksi']
        let version = saved.draft_version!
        for (const title of titles) {
          await moduleApi.addStep(props.orgId, saved.id, {
            ...blankStep(), title, body: 'Tulis materi di sini.',
            rich_content: templateRich('article'), expected_version: version++,
          })
          form.value = { ...form.value, expected_version: version }
          formBaseline.value = JSON.stringify(form.value)
        }
      }
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
 if(!window.confirm('Terbitkan revisi baru? Siswa lama tetap memakai revisinya. Kuis memakai revisi terbit, bukan draf bersama.')) return
 await run(()=>moduleApi.publish(props.orgId,item.id,item.draft_version!), 'Revisi modul diterbitkan.')
 try{history.value=(await moduleApi.history(props.orgId,item.id)).items}catch(err){error.value=errorMessage(err)}
}
async function archive(m: Module) {
  if (!window.confirm(`Arsipkan modul ${m.title}? Peserta tidak bisa membukanya lagi; file tetap ada.`)) return
  if (await run(() => moduleApi.archive(props.orgId, m.id), `${m.title} diarsipkan.`)) selected.value = null
}

async function fileOptions(offset: number) {
 const result = await fileApi.list(props.orgId, offset)
 return { items: result.items.map(f => ({ value: f.id, label: f.name })), more: hasMore(offset, result) }
}
async function quizOptions(offset: number) {
 const result = await quizApi.list(props.orgId, { building_key: active.value?.building_key, offset, limit: 20 })
 return { items: result.items.map(q => ({ value: q.id, label: `${q.title} (${q.level}, ${q.question_count} soal)` })), more: hasMore(offset, result) }
}
function editStep(m: Module, s?: ModuleStep) {
  stepSave.discard()
  stepVersion.value=m.draft_version!
  editing.value = null
  stepFor.value = m.id
  stepEditing.value = s?.id ?? 'new'
  fields.value = {}
  step.value = s
    ? { kind: s.kind, title: s.title, body: s.body, file_id: s.file_id, video_id: s.video_id ?? '', quiz_id: s.quiz_id, minutes: s.minutes, rich_content:s.rich_content }
    : blankStep()
  stepSave.reset()

}
async function saveStep() {
 await stepSave.flush(true)
 if(stepSave.state.value==='saved' && !stepSave.dirty.value){stepSave.discard();stepFor.value=null;message.value='Langkah disimpan.'}
}
function cancelStep(){if(stepSave.dirty.value && !window.confirm('Buang perubahan belum tersimpan?'))return;stepSave.discard();stepFor.value=null}
async function applyTemplate(kind: 'article'|'steps'|'code') {
 if(stepSave.dirty.value && !window.confirm('Ganti isi dengan struktur template?'))return
 step.value={...step.value,rich_content:templateRich(kind),body:kind==='article'?'Judul artikel':kind==='code'?'Penjelasan':'Tujuan'}
}
async function addStepTemplate(){
 const m=active.value;if(!m)return
 if(!window.confirm('Tambahkan struktur awal dua langkah? Isi materi masih harus ditulis.'))return
 await run(async()=>{
  let version=m.draft_version!
  for(const title of ['Tujuan belajar','Langkah pertama']){
   await moduleApi.addStep(props.orgId,m.id,{...blankStep(),title,body:'Tulis materi di sini.',rich_content:templateRich('article'),expected_version:version++})
  }
 },'Struktur awal ditambahkan. Isi materi sebelum terbit.')
}
async function archiveStep(m: Module, s: ModuleStep) {
  if (!window.confirm(`Arsipkan langkah ${s.title}?`)) return
  await run(() => moduleApi.archiveStep(props.orgId, m.id, s.id,m.draft_version), `Langkah ${s.title} diarsipkan.`)
}
async function move(m: Module, index: number, delta: -1 | 1) {
  const order = m.steps.map((s) => s.id)
  const [moved] = order.splice(index, 1)
  order.splice(index + delta, 0, moved!)
  await run(() => moduleApi.reorder(props.orgId, m.id, order,m.draft_version), `Urutan ${m.title} disimpan.`)
}

onMounted(async () => {
  await Promise.all([
    load(),
    fetchMap('campus').then((map) => (buildings.value = map.buildings)).catch((err) => (error.value = errorMessage(err))),
  ])
})
</script>

<template>
  <section class="manager authoring studio" :class="{ 'studio-active': selected || editing }" :aria-labelledby="`${id}-title`">
    <header v-if="!selected && !editing" class="head page-head">
      <div>
        <h2 :id="`${id}-title`" class="title">Modul belajar</h2>
        <p class="sub">Susun materi per gedung dan jenjang. Draf hanya terlihat pengajar; siswa membuka modul terbit dari gedungnya.</p>
      </div>
      <button v-if="!editing" type="button" class="btn-primary" :disabled="!choices.length" @click="edit()">+ Modul baru</button>
    </header>
    <p v-if="message" class="notice" role="status">{{ message }}</p>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>

    <header v-if="selected || (editing && editing !== 'new')" class="editor-head">
      <button type="button" class="btn-secondary" data-action="back" :disabled="busy" @click="back">Kembali ke daftar</button>
      <h3 ref="editorHeading" class="editor-title" tabindex="-1">{{ active?.title || 'Modul baru' }}</h3>
      <span v-if="active" class="chip">{{ active.published ? 'Terbit' : 'Draf' }}</span>
    </header>
    <div v-if="active && !editing && !stepFor" class="editor-actions actions">
      <button type="button" class="btn-secondary" :aria-pressed="preview" @click="preview = !preview">{{ preview ? 'Kembali menyusun' : 'Pratinjau isi' }}</button>
      <button type="button" class="btn-secondary" :disabled="busy" @click="edit(active)">Pengaturan</button>
      <button type="button" class="btn-primary" :disabled="busy || (!active.published && !active.steps.length)" @click="setPublished(active)">{{ active.published ? 'Terbitkan revisi baru' : 'Terbitkan' }}</button>
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
    <details v-if="active && history.length"><summary>Riwayat revisi terbit (hanya baca)</summary><ol><li v-for="r in history" :key="r.id">Revisi {{ r.revision }}</li></ol></details>
    <p v-if="preview" role="status">Pratinjau — belum diterbitkan</p>
    <p v-if="active && !active.published && !active.steps.length" class="hint">Tambahkan minimal satu materi sebelum menerbitkan.</p>
    <div v-if="!selected && !editing" class="catalog">
      <div class="toolbar">
        <StatusTabs v-model="statusFilter" :items="statusTabs" label="Status modul" :controls="`${id}-table`" />
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
      <label :for="`${id}-search`" class="sr">Cari judul modul</label>
      <input :id="`${id}-search`" v-model="search" type="search" class="search" placeholder="Cari judul modul…" />
      <p v-if="!visible.length" class="empty">{{ filtered ? 'Tidak ada modul yang cocok dengan filter ini.' : 'Belum ada modul. Mulai dari template agar siswa punya jalur belajar pertama.' }}
        <button v-if="!modules.length && choices.length" type="button" class="btn-secondary" @click="edit()">Buat modul pertama</button></p>
      <div v-else class="table-wrap">
        <table :id="`${id}-table`" class="data-table">
          <thead><tr><th scope="col">Modul</th><th scope="col">Gedung</th><th scope="col">Jenjang</th><th scope="col">Isi</th><th scope="col">Status</th><th scope="col"><span class="sr">Aksi</span></th></tr></thead>
          <tbody>
            <tr v-for="item in visible" :key="item.id" class="catalog-row">
              <th scope="row"><strong>{{ item.title }}</strong><small>{{ item.published ? item.description || 'Terlihat siswa' : 'Draf · belum terlihat siswa' }}</small></th>
              <td><span class="tag">{{ buildingName(item.building_key) }}</span></td>
              <td>{{ item.level }}</td>
              <td class="meta"><span class="phone-only">{{ buildingName(item.building_key) }} · {{ item.level }} · </span>{{ item.steps.length }} langkah</td>
              <td><span :class="['chip', statusFilter !== 'archived' && item.published ? 'ok' : 'draft']">{{ statusFilter === 'archived' ? 'Arsip' : item.published ? 'Terbit' : 'Draf' }}</span></td>
              <td v-if="statusFilter !== 'archived'" class="row-actions">
                <button v-if="item.published" type="button" class="btn-secondary soft" :aria-label="`Tugaskan ${item.title}`" @click="emit('assign', item.id)">Tugaskan</button>
                <button type="button" class="btn-secondary" :aria-label="`${item.published ? 'Buka' : 'Lanjutkan edit'} ${item.title}`" @click="openItem(item)">{{ item.published ? 'Buka' : 'Lanjutkan edit' }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <ListPagination v-if="!selected && !editing" v-model="page" :total="total" :size="pageSize" label="Halaman modul" />
    <div v-if="editing === 'new'" class="studio-start">
      <h2>Buat modul baru</h2>
      <p>Pilih cara paling mudah. Semua bagian bisa diubah lagi di editor.</p>
      <ol class="studio-stages" aria-label="Tahapan membuat modul"><li aria-current="step"><span>1</span>Mulai &amp; info dasar</li><li><span>2</span>Isi konten</li><li><span>3</span>Kuis</li><li><span>4</span>Terbitkan</li></ol>
    </div>
    <div v-if="editing" :class="{ 'new-layout': editing === 'new' }">
      <div v-if="editing === 'new'" class="new-main">
      <fieldset class="starter-options start-mode">
        <legend>Mulai dari</legend>
        <label :class="{ chosen: starter !== 'empty' }"><input type="radio" :name="`${id}-mode`" :checked="starter !== 'empty'" @change="starter = 'reading'" /><strong>Pakai template</strong><span>Struktur sudah jadi, tinggal isi tiap bagian.</span></label>
        <label class="disabled"><input type="radio" :name="`${id}-mode`" disabled /><strong>Upload file <small class="chip">Segera</small></strong><span>Pemecahan file otomatis belum tersedia. Lampirkan PDF dari Berkas di langkah modul.</span></label>
        <label :class="{ chosen: starter === 'empty' }"><input v-model="starter" type="radio" value="empty" :name="`${id}-mode`" /><strong>Mulai kosong</strong><span>Susun langkah sendiri dari nol.</span></label>
      </fieldset>
      <fieldset v-if="starter !== 'empty'" class="starter-options templates">
        <legend>Pilih template</legend>
        <label :class="['tpl-reading', { chosen: starter === 'reading' }]"><input v-model="starter" type="radio" value="reading" :name="`${id}-starter`" /><strong>Materi bacaan</strong><span>Pendahuluan · Materi · Contoh · Rangkuman</span><b>4 langkah</b></label>
        <label :class="['tpl-practice', { chosen: starter === 'practice' }]"><input v-model="starter" type="radio" value="practice" :name="`${id}-starter`" /><strong>Praktikum langkah demi langkah</strong><span>Tujuan · Alat dan bahan · Langkah · Hasil · Refleksi</span><b>5 langkah</b></label>
      </fieldset>
      </div>
    <form class="form studio-metadata" novalidate @submit.prevent="save">
      <h3 class="form-title">{{ editing === 'new' ? 'Info dasar' : 'Ubah modul' }}</h3>
      <div class="grid">
        <span class="group-label">Jenjang</span>
        <div class="choice-chips" role="radiogroup" aria-label="Jenjang">
          <label v-for="l in levels" :key="l" :class="{ chosen: form.level === l }"><input v-model="form.level" type="radio" :name="`${id}-l`" :value="l" />{{ l }}</label>
        </div>
        <span class="group-label">Gedung</span>
        <div class="choice-chips" role="radiogroup" aria-label="Gedung" :aria-invalid="!!fields.building_key">
          <label v-for="b in choices" :key="b.key" :class="{ chosen: form.building_key === b.key }"><input v-model="form.building_key" type="radio" :name="`${id}-b`" :value="b.key" />{{ b.name }}</label>
        </div>
        <p v-if="fields.building_key" class="field-error">{{ fields.building_key }}</p>
        <label :for="`${id}-t`">Judul modul</label>
        <input :id="`${id}-t`" v-model="form.title" maxlength="120" placeholder="Contoh: Penjumlahan sampai 20" :aria-invalid="!!fields.title"
          :aria-describedby="fields.title ? `${id}-te` : undefined" />
        <p v-if="fields.title" :id="`${id}-te`" class="field-error">{{ fields.title }}</p>
        <label :for="`${id}-d`">Deskripsi (opsional)</label>
        <textarea :id="`${id}-d`" v-model="form.description" maxlength="500" rows="3" placeholder="Apa yang akan dipelajari siswa?" />
      </div>
      <div v-if="formConflict" class="actions"><button type="button" class="btn-secondary" @click="copyLocal">Salin perubahan lokal</button><button type="button" class="btn-secondary" @click="reloadForm">Muat versi server</button></div>
      <p v-if="editing === 'new'" class="hint">Disimpan sebagai draf. Tambahkan isi, lalu terbitkan.</p>
      <div class="actions">
        <button type="button" class="btn-secondary" :disabled="busy" @click="mayLeave() && (editing = null)">Batal</button>
        <button type="submit" class="btn-primary" :disabled="busy">{{ editing === 'new' ? 'Lanjut isi konten' : 'Simpan' }}</button>
      </div>
    </form>
    </div>

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
      <div class="studio-workspace" :class="{ 'is-editing': !!stepFor }">
      <nav v-if="!preview" class="studio-outline" aria-label="Bagian modul">
        <h4>Bagian modul</h4>
        <button v-for="(s, i) in m.steps" :key="s.id" type="button" :aria-current="stepEditing === s.id && stepFor ? 'step' : undefined" :disabled="busy" @click="mayLeave() && editStep(m, s)"><span class="outline-number">{{ i + 1 }}</span>{{ s.title }}</button>
        <button type="button" class="outline-add" :disabled="busy" @click="mayLeave() && editStep(m)">+ Tambah bagian</button>
      </nav>
      <div class="studio-content">
      <ol v-if="!stepFor" class="steps">
        <li v-for="(s, i) in m.steps" :key="s.id">
          <span class="step-title"><strong>{{ i + 1 }}. {{ s.title }}</strong> <small>{{ kindLabel[s.kind] }} · {{ s.minutes }} mnt</small><RichContent v-if="preview && s.rich_content" :content="s.rich_content" /><p v-else-if="preview" class="preview-body">{{ s.body }}</p></span>
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
            <PagedSelect :id="`${id}-sf`" :model-value="step.file_id ?? ''" label="Pilih berkas" :load="fileOptions" @update:model-value="step.file_id = $event || null" />
            <p :class="fields.file_id ? 'field-error' : 'hint'">
              {{ fields.file_id || 'PDF atau audio dari File belajar. Ganti halaman pilihan untuk berkas lainnya.' }}
            </p>
          </template>
          <template v-if="step.kind === 'kuis'">
            <label :for="`${id}-sq`">Kuis</label>
            <PagedSelect :id="`${id}-sq`" :model-value="step.quiz_id ?? ''" label="Kuis" :load="quizOptions" @update:model-value="step.quiz_id = $event || null" />
            <p :class="fields.quiz_id ? 'field-error' : 'hint'">
              {{ fields.quiz_id || 'Selesai otomatis saat peserta lulus kuis. Pilihan hanya kuis terbit di gedung ini.' }}
            </p>
          </template>
          <template v-if="step.kind === 'video'">
            <label :for="`${id}-sv`">Tautan YouTube</label>
            <input :id="`${id}-sv`" v-model="step.video_id" inputmode="url" placeholder="https://youtu.be/…"
              :aria-invalid="!!fields.video_id" />
            <p :class="fields.video_id ? 'field-error' : 'hint'">{{ fields.video_id || 'Diputar di dalam aplikasi lewat youtube-nocookie.com.' }}</p>
          </template>
          <label :for="`${id}-sb`">{{ step.kind === 'materi' ? 'Isi materi' : 'Catatan (opsional)' }}</label>
          <div v-if="step.kind==='materi'" class="rich-form">
            <div class="actions" aria-label="Template awal">
              <button type="button" class="btn-secondary" @click="applyTemplate('article')">Artikel</button>
              <button type="button" class="btn-secondary" @click="applyTemplate('steps')">Langkah demi langkah</button>
              <button type="button" class="btn-secondary" @click="applyTemplate('code')">Tutorial kode</button>
            </div>
            <RichEditor v-model="step.rich_content" v-model:text="step.body!" />
          </div>
          <textarea v-else :id="`${id}-sb`" v-model="step.body" maxlength="5000" rows="2" :aria-invalid="!!fields.body" />
          <p v-if="fields.body" class="field-error">{{ fields.body }}</p>
          <label :for="`${id}-sm`">Durasi (menit)</label>
          <input :id="`${id}-sm`" v-model.number="step.minutes" type="number" min="1" max="240" inputmode="numeric"
            :aria-invalid="!!fields.minutes" />
          <p v-if="fields.minutes" class="field-error">{{ fields.minutes }}</p>
        </div>
        <p role="status">{{ stepSave.label.value }}</p>
        <p v-if="stepSave.error.value" role="alert">{{ stepSave.error.value }}</p>
        <div v-if="stepSave.state.value==='conflict'" class="actions">
          <button type="button" class="btn-secondary" @click="copyDraft">Salin perubahan lokal</button>
          <button type="button" class="btn-secondary" @click="reloadDraft">Muat versi server</button>
        </div>
        <button v-if="stepSave.state.value==='failed'" type="button" class="btn-secondary" @click="stepSave.flush()">Coba simpan lagi</button>
        <p v-if="fields.steps" class="field-error">{{ fields.steps }}</p>
        <div class="actions">
          <button type="submit" class="btn-primary" :disabled="busy">Simpan langkah</button>
          <button type="button" class="btn-secondary" :disabled="busy" @click="cancelStep">Batal</button>
        </div>
      </form>
      </div>
      <aside v-if="!preview" class="studio-settings" aria-label="Pengaturan modul">
        <h4>Kelengkapan modul</h4>
        <progress :value="m.steps.filter(s => s.body || s.file_id || s.video_id || s.quiz_id).length" :max="Math.max(1, m.steps.length)" aria-label="Bagian berisi konten" />
        <p>{{ m.steps.length }} bagian · {{ m.steps.reduce((total, s) => total + s.minutes, 0) }} menit</p>
        <p>{{ m.steps.some(s => s.kind === 'kuis') ? '✓ Kuis terhubung' : 'Belum ada kuis terhubung' }}</p>
        <h4>Pengaturan</h4><p>{{ buildingName(m.building_key) }} · {{ m.level }}</p>
        <p class="hint">Perubahan disimpan sebagai draf. Terbitkan revisi baru agar tersedia bagi peserta baru.</p>
      </aside>
      </div>
      <button v-if="!preview && !stepFor" type="button" class="btn-secondary" :disabled="busy" @click="addStepTemplate">Template beberapa langkah</button>
      <button v-if="!preview && !stepFor" type="button" class="btn-primary add" :disabled="busy" @click="editStep(m)">+ Langkah<span class="sr"> di {{ m.title }}</span></button>
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
