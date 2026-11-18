<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { assignmentApi, deadline, type Assignment, type AssignmentRecipient, type AssignmentInput } from '@/api/assignments'
import { classApi, type AcademicYear, type SchoolClass } from '@/api/classes'
import { moduleApi } from '@/api/modules'
import type { Module } from '@/api/types'
import { ApiError, errorMessage } from '@/api/client'
const props = defineProps<{ orgId: string; moduleId?: string }>()
const years = ref<AcademicYear[]>([])
const year = ref('')
const classes = ref<SchoolClass[]>([])
const modules = ref<Module[]>([])
const tasks = ref<Assignment[]>([])
const classID = ref('')
const moduleID = ref(props.moduleId ?? '')
const due = ref('')
const roster = ref<AssignmentRecipient[]>([])
const rosterTitle = ref('')
const busy = ref(false)
const error = ref('')
const pending = ref<AssignmentInput | null>(null)
const pendingKey = `kv-assignment-pending:${props.orgId}`
let generation = 0
onBeforeUnmount(() => { generation++ })
async function load() {
  const g = ++generation; busy.value = true; error.value = ''
  try {
    const [y, m, a] = await Promise.all([classApi.years(props.orgId), moduleApi.list(props.orgId), assignmentApi.list(props.orgId)])
    if (g !== generation) return
    years.value = y.items; modules.value = m.items.filter(m => m.published && m.steps.length > 0); tasks.value = a.items
    year.value = y.items.find(y => y.id === year.value)?.id ?? y.items.find(y => y.active)?.id ?? y.items[0]?.id ?? ''
    if (year.value) {
      const c = await classApi.classes(props.orgId, year.value)
      if (g === generation) classes.value = c.items
    } else classes.value = []
  } catch (e) { if (g === generation) error.value = errorMessage(e) }
  finally { if (g === generation) busy.value = false }
}
async function changeYear() {
  classID.value = ''; classes.value = []; busy.value = true; error.value = ''
  const g = ++generation
  try { const result = await classApi.classes(props.orgId, year.value); if (g === generation) classes.value = result.items }
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
    sessionStorage.removeItem(pendingKey); pending.value = null; rosterTitle.value = ''; await load()
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
        pending.value = saved; classID.value = saved.class_id; moduleID.value = saved.module_id
      } else sessionStorage.removeItem(pendingKey)
    }
  } catch { error.value = 'Pemulihan tugas tertunda gagal. Periksa penyimpanan browser.' }
  void load()
})
</script>
<template>
  <section class="assignment-manager" aria-labelledby="assignment-heading" :aria-busy="busy">
    <h2 id="assignment-heading">Tugaskan modul ke kelas</h2>
    <p>Penerima dan isi langkah dibekukan saat tugas dibuat. Pindah kelas tidak menghapus tugas; siswa baru tidak otomatis ditambahkan. Katalog tetap terbuka.</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <button type="button" :disabled="busy" @click="load">Muat ulang</button>
    <p v-if="busy" role="status">Memuat tugas…</p>
    <form @submit.prevent="issue">
      <fieldset :disabled="busy || !!pending">
        <legend>Tugas baru</legend>
        <label>Tahun ajaran <select v-model="year" required @change="changeYear"><option value="" disabled>Pilih tahun</option><option v-for="y in years" :key="y.id" :value="y.id">{{ y.label }}</option></select></label>
        <label>Kelas <select v-model="classID" required><option value="" disabled>Pilih kelas</option><option v-for="c in classes" :key="c.id" :value="c.id" :disabled="!c.students">{{ c.name }} · {{ c.students }} siswa</option></select></label>
        <label>Modul terbit <select v-model="moduleID" required><option value="" disabled>Pilih modul</option><option v-for="m in modules" :key="m.id" :value="m.id">{{ m.title }} · {{ m.level }}</option></select></label>
        <label>Tenggat opsional (WIB) <input v-model="due" type="datetime-local" /></label>
      </fieldset>
      <p>Urutan langkah wajib; progres dimulai dari nol. Tenggat hanya penanda, bukan pengunci pengerjaan.</p>
      <button type="submit" :disabled="busy || (!pending && (!classID || !moduleID))">{{ pending ? 'Coba kirim tugas yang sama' : 'Tugaskan' }}</button>
      <p v-if="pending">Hasil pengiriman belum dipastikan. Coba ulang dengan ID sama agar tidak membuat tugas ganda.</p>
    </form>
    <h3>Tugas sekolah</h3>
    <ul><li v-for="a in tasks" :key="a.id">
      <strong>{{ a.title }}</strong> · {{ a.class_name }} · {{ a.year_label }} · {{ deadline(a.due_at) }} {{ a.cancelled ? '· Dibatalkan' : '' }}
      <div class="actions"><button type="button" :disabled="busy" @click="progress(a)">Progres siswa</button><button v-if="!a.cancelled" type="button" :disabled="busy" @click="cancel(a)">Batalkan tugas</button></div>
    </li></ul>
    <p v-if="!tasks.length && !busy">Belum ada tugas.</p>
    <section v-if="rosterTitle" aria-live="polite"><h3>Progres: {{ rosterTitle }}</h3><ul><li v-for="r in roster" :key="r.user_id">{{ r.display_name }} · {{ r.done }}/{{ r.total }} selesai</li></ul></section>
  </section>
</template>
<style scoped>
.assignment-manager { padding: var(--space-6); margin-top: var(--space-6); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); background: var(--color-surface); overflow-wrap: anywhere; }
h2, h3 { font-family: var(--font-display); }
fieldset { display: flex; flex-wrap: wrap; gap: var(--space-4); border: 0; padding: 0; min-width: 0; }
label { display: flex; flex: 1; flex-direction: column; gap: var(--space-2); min-width: min(100%, 200px); }
input, select, button { min-height: var(--touch-min); font: inherit; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: var(--space-2); max-width: 100%; color: var(--color-ink); background: var(--color-surface); box-sizing: border-box; }
button { cursor: pointer; } button:disabled { opacity: .6; cursor: not-allowed; }
li { margin-block: var(--space-4); }
.actions { display: flex; flex-wrap: wrap; gap: var(--space-2); margin-top: var(--space-2); }
</style>
