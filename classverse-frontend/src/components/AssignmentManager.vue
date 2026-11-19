<script setup lang="ts">
import { hasMore } from '@/api/page'
import PagedSelect from './PagedSelect.vue'
import RemotePagination from './RemotePagination.vue'
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { assignmentApi, deadline, type Assignment, type AssignmentRecipient, type AssignmentInput } from '@/api/assignments'
import { classApi, type AcademicYear, type SchoolClass } from '@/api/classes'
import { moduleApi } from '@/api/modules'
import { ApiError, errorMessage } from '@/api/client'
const props = defineProps<{ orgId: string; moduleId?: string }>()
const years = ref<AcademicYear[]>([])
const year = ref('')
const classes = ref<SchoolClass[]>([])
const tasks = ref<Assignment[]>([])
const classID = ref('')
const moduleID = ref(props.moduleId ?? '')
const due = ref('')
const panelOpen = ref(!!props.moduleId)
const panel = ref<HTMLElement | null>(null)
const newButton = ref<HTMLButtonElement | null>(null)
const search = ref('')
const classFilter = ref('')
// Search and the class filter run on the server before paging.
const visibleTasks = tasks
const moreTasks = ref(false)
const pageNumber = ref(1)
function movePage(delta: number) { pageNumber.value += delta; void load() }
watch([search, classFilter], () => { pageNumber.value = 1; void load() })
async function openPanel() { panelOpen.value = true; await nextTick(); panel.value?.focus() }
function closePanel() { panelOpen.value = false; newButton.value?.focus() }
const roster = ref<AssignmentRecipient[]>([])
const rosterTitle = ref('')
const busy = ref(false)
const error = ref('')
const pending = ref<AssignmentInput | null>(null)
const pendingKey = `kv-assignment-pending:${props.orgId}`
let generation = 0
onBeforeUnmount(() => { generation++ })
async function classFilterOptions(offset: number) {
 if (!year.value) return { items: [], more: false }
 const result = await classApi.classes(props.orgId, year.value, offset)
 return { items: result.items.map(c => ({ value: c.id, label: c.name })), more: hasMore(offset, result) }
}
async function classOptions(offset: number) {
 if (!year.value) return { items: [], more: false }
 const result = await classApi.classes(props.orgId, year.value, offset)
 return { items: result.items.map(c => ({ value: c.id, label: `${c.name} · ${c.students} siswa${!c.students ? ' · Belum bisa ditugaskan' : ''}`, disabled: !c.students })), more: hasMore(offset, result) }
}
async function moduleOptions(offset: number) {
 const result = await moduleApi.list(props.orgId, { status: 'published', limit: 20, offset })
 // Published only (server filter); a module without steps stays visible but disabled so no page is emptied.
 return { items: result.items.map(m => ({ value: m.id, label: `${m.title} · ${m.level}${m.steps.length ? '' : ' · Belum ada langkah'}`, disabled: !m.steps.length })), more: hasMore(offset, result) }
}
async function yearOptions(offset: number) {
 const result = await classApi.years(props.orgId, offset)
 years.value = [...years.value.filter(y => !result.items.some(n => n.id === y.id)), ...result.items]
 return { items: result.items.map(y => ({ value: y.id, label: y.label + (y.active ? ' · Aktif' : '') })), more: hasMore(offset, result) }
}
async function load() {
  const g = ++generation; busy.value = true; error.value = ''
  try {
    const [y, a] = await Promise.all([classApi.years(props.orgId), assignmentApi.list(props.orgId, (pageNumber.value - 1) * 20, search.value.trim(), classFilter.value)])
    if (g !== generation) return
    years.value = y.items; tasks.value = a.items; moreTasks.value = hasMore((pageNumber.value - 1) * 20, a)
    year.value = year.value || y.items.find(y => y.active)?.id || y.items[0]?.id || ''
    if (year.value) {
      const c = await classApi.classes(props.orgId, year.value, 0)
      if (g === generation) classes.value = c.items
    } else classes.value = []
  } catch (e) { if (g === generation) error.value = errorMessage(e) }
  finally { if (g === generation) busy.value = false }
}
async function changeYear() {
  classID.value = ''; classFilter.value = ''; classes.value = []; busy.value = true; error.value = ''
  const g = ++generation
  try { const result = await classApi.classes(props.orgId, year.value, 0); if (g === generation) classes.value = result.items }
  catch (e) { if (g === generation) error.value = errorMessage(e) }
  finally { if (g === generation) busy.value = false }
}
async function issue() {
  if (busy.value) return
  if (!pending.value) pending.value = { id: crypto.randomUUID(), class_id: classID.value, module_id: moduleID.value, due_at: due.value ? new Date(`${due.value}:00+07:00`).toISOString() : null }
  busy.value = true; error.value = ''
  try {
    sessionStorage.setItem(pendingKey, JSON.stringify(pending.value))
    await assignmentApi.issue(props.orgId, pending.value)
    sessionStorage.removeItem(pendingKey); pending.value = null; rosterTitle.value = ''; closePanel(); await load()
  }
  catch (e) {
    error.value = errorMessage(e)
    // A definitive rejection did not issue a task; let staff correct its fields.
    if (e instanceof ApiError && [400, 403, 404, 409].includes(e.status)) {
      pending.value = null; sessionStorage.removeItem(pendingKey)
    }
  }
  finally { busy.value = false }
}
async function progress(a: Assignment) {
  busy.value = true; error.value = ''; rosterTitle.value = ''
  try { roster.value = (await assignmentApi.roster(props.orgId, a.id)).items; rosterTitle.value = a.title }
  catch (e) { error.value = errorMessage(e) }
  finally { busy.value = false }
}
async function cancel(a: Assignment) {
  if (!window.confirm(`Batalkan tugas ${a.title}? Pengerjaan dihentikan; hasil dan riwayat tetap tersimpan.`)) return
  busy.value = true; error.value = ''
  try { await assignmentApi.cancel(props.orgId, a.id); await load() }
  catch (e) { error.value = errorMessage(e) }
  finally { busy.value = false }
}
onMounted(() => {
  try {
    const raw = sessionStorage.getItem(pendingKey)
    if (raw) {
      const saved = JSON.parse(raw) as AssignmentInput
      if (typeof saved.id === 'string' && typeof saved.class_id === 'string' && typeof saved.module_id === 'string' && (saved.due_at == null || typeof saved.due_at === 'string')) {
        pending.value = saved; classID.value = saved.class_id; moduleID.value = saved.module_id; panelOpen.value = true
      } else sessionStorage.removeItem(pendingKey)
    }
  } catch { error.value = 'Pemulihan tugas tertunda gagal. Periksa penyimpanan browser.' }
  void load()
})
</script>
<template>
  <section class="assignment-manager" aria-labelledby="assignment-heading" :aria-busy="busy">
    <header class="page-head"><div><h2 id="assignment-heading">Tugas</h2>
    <p>Tugaskan modul terbit ke satu kelas dan pantau progres siswa per langkah.</p></div>
    <button ref="newButton" type="button" class="btn-primary" data-action="new-task" @click="openPanel">+ Tugas baru</button></header>
    <p v-if="error" role="alert">{{ error }}</p>
    <button type="button" :disabled="busy" @click="load">Muat ulang</button>
    <p v-if="busy" role="status">Memuat tugas…</p>
    <div class="task-layout" :class="{ 'with-panel': panelOpen }">
    <div class="task-list">
      <div class="toolbar">
        <PagedSelect v-model="classFilter" :label="`Filter kelas ${years.find(y => y.id === year)?.label ?? ''}`.trim()" placeholder="Semua kelas" :key="`filter-${year}`" :load="classFilterOptions" />
        <label>Cari tugas <input v-model="search" type="search" placeholder="Cari tugas…" /></label>
      </div>
      <ul class="tasks"><li v-for="a in visibleTasks" :key="a.id">
        <strong>{{ a.title }}</strong><p>{{ a.class_name }} · {{ a.year_label }} · {{ deadline(a.due_at) }} {{ a.cancelled ? '· Dibatalkan' : '' }}</p>
        <template v-if="a.recipients">
          <progress :value="a.finished ?? 0" :max="a.recipients" :aria-label="`Siswa selesai ${a.title}`" />
          <p class="counts">{{ a.finished }} selesai · {{ (a.started ?? 0) - (a.finished ?? 0) }} sedang · {{ a.recipients - (a.started ?? 0) }} belum mulai</p>
        </template>
        <p v-else class="counts">Tidak ada penerima.</p>
        <div class="actions"><button type="button" :disabled="busy" @click="progress(a)">Progres siswa</button><button v-if="!a.cancelled" type="button" :disabled="busy" @click="cancel(a)">Batalkan tugas</button></div>
      </li></ul>
      <RemotePagination v-if="pageNumber > 1 || moreTasks" :page="pageNumber" :more="moreTasks" :busy="busy" @move="movePage" />
      <div v-if="!visibleTasks.length && !busy" class="empty"><h3>{{ search || classFilter ? 'Tidak ada tugas yang cocok' : 'Belum ada tugas' }}</h3><p>Tugas muncul di panel “Tugas kelas” siswa. Progres dihitung per langkah modul dan dapat dipantau di sini.</p></div>
      <section v-if="rosterTitle" aria-live="polite"><h3>Progres: {{ rosterTitle }}</h3><ul><li v-for="r in roster" :key="r.user_id">{{ r.display_name }} · {{ r.done }}/{{ r.total }} selesai</li></ul></section>
    </div>
    <aside v-if="panelOpen" ref="panel" tabindex="-1" class="task-panel" aria-labelledby="new-task-heading" @keydown.esc="closePanel">
    <header class="page-head"><h3 id="new-task-heading">Tugas baru</h3><button type="button" aria-label="Tutup tugas baru" @click="closePanel">Tutup</button></header>
    <form @submit.prevent="issue">
      <fieldset :disabled="busy || !!pending">
        <legend class="sr-only">Pilih penerima dan modul</legend>
        <PagedSelect v-model="year" label="Tahun ajaran" :load="yearOptions" @update:model-value="changeYear" />
        <PagedSelect v-model="classID" label="1 · Kelas" placeholder="Pilih kelas" :load="classOptions" :key="year" :disabled="busy || !year" />
        <PagedSelect v-model="moduleID" label="2 · Modul terbit" placeholder="Pilih modul" :load="moduleOptions" :disabled="busy" />
        <label>3 · Tenggat opsional (WIB) <input v-model="due" type="datetime-local" /></label>
      </fieldset>
      <p>Tenggat hanya penanda, siswa tetap bisa mengerjakan setelahnya.</p>
      <div class="freeze-notice"><strong>Sebelum menugaskan</strong><p>Penerima dan isi langkah dibekukan saat tugas dibuat. Pindah kelas tidak menghapus tugas; siswa baru tidak otomatis ditambahkan. Urutan langkah wajib; progres dimulai dari nol.</p></div>
      <button type="submit" :disabled="busy || (!pending && (!classID || !moduleID))">{{ pending ? 'Coba kirim tugas yang sama' : `Tugaskan${classes.find(c => c.id === classID)?.name ? ' ke ' + classes.find(c => c.id === classID)!.name : ''}` }}</button>
      <p v-if="pending">Hasil pengiriman belum dipastikan. Coba ulang dengan ID sama agar tidak membuat tugas ganda.</p>
    </form>
    </aside>
    </div>
  </section>
</template>
<style scoped>
.assignment-manager { padding: var(--space-5); background: var(--kv-page); border-radius: var(--radius-2xl); overflow-wrap: anywhere; }
.page-head { display: flex; flex-wrap: wrap; gap: var(--space-3); justify-content: space-between; align-items: start; }
.page-head h2, .page-head h3 { margin: 0; }
.page-head p { color: var(--kv-text-muted); }
.task-layout { display: grid; gap: var(--space-5); margin-top: var(--space-4); }
.task-layout.with-panel { grid-template-columns: minmax(0, 1fr) 340px; align-items: start; }
.task-list { min-width: 0; }
.task-panel { padding: var(--space-5); border: 1px solid var(--kv-line); border-radius: var(--radius-2xl); background: var(--kv-surface); min-width: 0; }
.task-panel fieldset { flex-direction: column; margin-top: var(--space-4); }
.task-panel label { min-width: 0; width: 100%; }
.task-panel select, .task-panel input { width: 100%; min-width: 0; }
.task-panel { box-sizing: border-box; }
.toolbar { display: flex; flex-wrap: wrap; gap: var(--space-3); }
.tasks { padding: 0; list-style: none; }
.tasks progress { width: 100%; height: 8px; accent-color: var(--kv-primary); }
.counts { margin: var(--space-1) 0 0; color: var(--kv-text-muted); font-size: var(--text-sm); }
.tasks > li { padding: var(--space-4); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); background: var(--kv-surface); }
.empty { padding: var(--space-6); border: 1px solid var(--kv-line); border-radius: var(--radius-2xl); text-align: center; background: var(--kv-surface); color: var(--kv-text-muted); }
.freeze-notice { background: var(--kv-primary-soft); color: var(--kv-primary-soft-text); padding: var(--space-4); border-radius: var(--radius-lg); margin-block: var(--space-4); }
.freeze-notice p { margin-bottom: 0; }
button:focus-visible, .task-panel:focus-visible { outline: 2px solid var(--kv-primary); outline-offset: 2px; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
@media (max-width: 1000px) { .task-layout.with-panel { grid-template-columns: minmax(0, 1fr); } .task-panel { grid-row: 1; } }
@media (max-width: 700px) { .assignment-manager { padding: var(--space-3); } }
h2, h3 { font-family: var(--font-display); }
fieldset { display: flex; flex-wrap: wrap; gap: var(--space-4); border: 0; padding: 0; min-width: 0; }
label { display: flex; flex: 1; flex-direction: column; gap: var(--space-2); min-width: min(100%, 200px); }
input, select, button { min-height: var(--touch-min); font: inherit; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: var(--space-2); max-width: 100%; color: var(--color-ink); background: var(--color-surface); box-sizing: border-box; }
button { cursor: pointer; } button:disabled { opacity: .6; cursor: not-allowed; }
li { margin-block: var(--space-4); }
.actions { display: flex; flex-wrap: wrap; gap: var(--space-2); margin-top: var(--space-2); }
</style>
