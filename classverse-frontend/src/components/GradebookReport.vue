<script setup lang="ts">
import { hasMore } from '@/api/page'
import PagedSelect from './PagedSelect.vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useOrgStore } from '@/stores/org'
import { errorMessage } from '@/api/client'
import { gradebookApi, type GradebookPage, type ReportFilter, type StudentGradebook } from '@/api/gradebook'
import { classApi, type AcademicYear, type SchoolClass } from '@/api/classes'
import { deadline } from '@/api/assignments'

defineProps<{ embedded?: boolean }>()
const org = useOrgStore()
const orgId = computed(() => org.active?.id ?? '')
const staff = computed(() => org.isTeacher)
const filter = ref<ReportFilter>({})
const years = ref<AcademicYear[]>([])
const classes = ref<SchoolClass[]>([])
const page = ref<GradebookPage | null>(null)
const offset = ref(0)
const report = ref<StudentGradebook | null>(null)
const busy = ref(false)
const error = ref('')
let generation = 0
onBeforeUnmount(() => { generation++ })
const score = (v: number | null | undefined) => v == null ? 'Belum dikerjakan' : String(Math.round(v * 100) / 100)

async function classOptions(offset: number) {
 if (!filter.value.year_id) return { items: [], more: false }
 const result = await classApi.classes(orgId.value, filter.value.year_id, offset)
 return { items: result.items.map(c => ({ value: c.id, label: c.name })), more: hasMore(offset, result) }
}
async function yearOptions(offset: number) {
 const result = await classApi.years(orgId.value, offset)
 years.value = [...years.value.filter(y => !result.items.some(n => n.id === y.id)), ...result.items]
 return { items: result.items.map(y => ({ value: y.id, label: y.label + (y.active ? ' · Aktif' : '') })), more: hasMore(offset, result) }
}
async function load() {
  if (!orgId.value) return
  const g = ++generation; busy.value = true; error.value = ''
  try {
    if (staff.value) {
      const result = await gradebookApi.roster(orgId.value, filter.value, offset.value)
      if (g === generation) page.value = result
    } else {
      const result = await gradebookApi.me(orgId.value, filter.value)
      if (g === generation) report.value = result
    }
  } catch (e) { if (g === generation) error.value = errorMessage(e) }
  finally { if (g === generation) busy.value = false }
}
async function loadYears() {
  if (!staff.value || !orgId.value) return
  try {
    years.value = (await classApi.years(orgId.value)).items
    const active = years.value.find(y => y.active)
    if (active) { filter.value = { ...filter.value, year_id: active.id }; classes.value = (await classApi.classes(orgId.value, active.id)).items }
  } catch (e) { error.value = errorMessage(e) }
}
async function changeYear() {
  filter.value = { ...filter.value, class_id: undefined }
  classes.value = []
  if (filter.value.year_id) {
    try { classes.value = (await classApi.classes(orgId.value, filter.value.year_id)).items }
    catch (e) { error.value = errorMessage(e); return }
  }
  apply()
}
function apply() { offset.value = 0; report.value = null; void load() }
function move(step: number) { offset.value = Math.max(0, offset.value + step * 20); void load() }
async function openStudent(user: string) {
  const g = ++generation
  busy.value = true; error.value = ''
  try {
    const result = await gradebookApi.student(orgId.value, user, filter.value)
    if (g === generation) report.value = result
  }
  catch (e) { if (g === generation) error.value = errorMessage(e) }
  finally { if (g === generation) busy.value = false }
}
async function download() {
  busy.value = true; error.value = ''
  try {
    const url = URL.createObjectURL(await gradebookApi.csv(orgId.value, filter.value))
    const a = document.createElement('a'); a.href = url; a.download = 'rapor.csv'; a.click()
    setTimeout(() => URL.revokeObjectURL(url), 0)
  } catch (e) { error.value = errorMessage(e) }
  finally { busy.value = false }
}
watch(orgId, async () => { filter.value = {}; page.value = null; report.value = null; await loadYears(); await load() })
onMounted(async () => { await loadYears(); await load() })
</script>

<template>
    <section class="gradebook" :aria-busy="busy" aria-labelledby="gradebook-title">
      <header class="page-head">
        <div><component :is="embedded ? 'h2' : 'h1'" id="gradebook-title" class="title">Rapor belajar</component>
        <p class="sub muted">Pantau progres modul, tugas, dan nilai kuis tertinggi. Nilai akademik 0–100 terpisah dari poin.</p></div>
        <button v-if="staff" type="button" class="btn-secondary" :disabled="busy" @click="download">Unduh CSV</button>
      </header>
      <p v-if="error" role="alert">{{ error }} <button type="button" @click="load">Coba lagi</button></p>
      <p v-if="busy" role="status">Memuat rapor…</p>
      <form v-if="staff" class="filters" aria-label="Filter rapor" @submit.prevent="apply">
        <PagedSelect :model-value="filter.year_id ?? ''" label="Tahun ajaran" placeholder="Semua tahun" :load="yearOptions" @update:model-value="filter.year_id = $event || undefined; changeYear()" />
        <PagedSelect :model-value="filter.class_id ?? ''" label="Kelas" placeholder="Semua kelas" :key="filter.year_id" :load="classOptions" :disabled="!filter.year_id" @update:model-value="filter.class_id = $event || undefined" />
        <label>Gedung (kunci)
          <input v-model.trim="filter.building_key" maxlength="40" placeholder="mis. sains" />
        </label>
        <div class="actions"><button type="submit" class="btn-primary" :disabled="busy">Terapkan filter</button></div>
        <p class="muted">Filter kelas memilih siswa yang sedang ditempatkan pada tahun terpilih; hasil lama tidak dipindahkan ke kelas baru.</p>
      </form>
      <p v-if="staff && page && !page.items.length && !busy" class="empty" role="status">Belum ada peserta untuk filter ini. Pilih tahun atau kelas lain.</p>
      <div v-if="staff && page && !report && page.items.length" class="table-wrap">
      <table>
        <caption>{{ page.total }} peserta</caption>
        <thead><tr><th scope="col">Nama</th><th scope="col">Kelas</th><th scope="col">Modul selesai</th><th scope="col">Kuis dikerjakan</th><th scope="col">Rata-rata terbaik</th><th scope="col"><span class="sr-only">Aksi</span></th></tr></thead>
        <tbody>
          <tr v-for="r in page.items" :key="r.user_id">
            <th scope="row">{{ r.name }}</th><td>{{ r.class_name || 'Belum ditempatkan' }}</td>
            <td>{{ r.modules_done }}/{{ r.modules_total }}</td><td>{{ r.quizzes_attempted }}/{{ r.quizzes_total }}</td>
            <td>{{ score(r.average_best) }}</td>
            <td><button type="button" :aria-label="`Lihat rapor ${r.name}`" @click="openStudent(r.user_id)">Detail</button></td>
          </tr>
        </tbody>
      </table>
      </div>
      <nav v-if="staff && page && !report && page.items.length" class="actions" aria-label="Halaman rapor">
        <button type="button" :disabled="busy || offset === 0" @click="move(-1)">Sebelumnya</button>
        <button type="button" :disabled="busy || offset + 20 >= page.total" @click="move(1)">Berikutnya</button>
      </nav>
      <section v-if="report" class="student" aria-live="polite">
        <button v-if="staff" type="button" class="btn-secondary" aria-label="Kembali ke daftar rapor" @click="report = null">Kembali ke daftar</button>
        <h2>{{ report.name }}</h2>
        <p>Rata-rata nilai terbaik: <strong>{{ score(report.average_best) }}</strong></p>
        <h3>Modul katalog</h3>
        <ul><li v-for="m in report.modules" :key="m.id">{{ m.title }} · {{ m.building_key }} · {{ m.done }}/{{ m.total }} langkah{{ m.archived ? ' · Diarsipkan' : '' }}</li></ul>
        <p v-if="!report.modules.length">Belum ada modul.</p>
        <h3>Kuis</h3>
        <ul><li v-for="q in report.quizzes" :key="q.id">{{ q.title }} · terbaik {{ score(q.best) }} · terakhir {{ score(q.last) }} · {{ q.attempts }} percobaan{{ q.archived ? ' · Diarsipkan' : '' }}</li></ul>
        <p v-if="!report.quizzes.length">Belum ada kuis.</p>
        <h3>Tugas kelas</h3>
        <ul><li v-for="a in report.assignments" :key="a.id">{{ a.title }} · {{ a.class_name }} {{ a.year_label }} · {{ a.done }}/{{ a.total }} · {{ deadline(a.due_at) }}{{ a.cancelled ? ' · Dibatalkan' : '' }}</li></ul>
        <p v-if="!report.assignments.length">Belum ada tugas.</p>
      </section>
    </section>
</template>

<style scoped>
.gradebook { min-width: 0; padding: var(--space-5); background: var(--kv-page); border-radius: var(--radius-2xl); }
.page-head { display: flex; flex-wrap: wrap; gap: var(--space-4); justify-content: space-between; align-items: start; }
.page-head .title { margin: 0; font-size: var(--text-3xl); }
.page-head .sub { max-width: 70ch; }
.table-wrap { background: var(--kv-surface); border: 1px solid var(--kv-line); border-radius: var(--radius-2xl); overflow-x: auto; }
caption { text-align: left; padding: var(--space-4); font-weight: var(--weight-semibold); }
thead { background: var(--kv-surface-soft); color: var(--kv-text-muted); font-size: var(--text-xs); }
.empty { padding: var(--space-6); border: 1px dashed var(--kv-line); border-radius: var(--radius-lg); text-align: center; }
button:focus-visible { outline: 2px solid var(--kv-primary); outline-offset: 2px; }
@media (max-width: 700px) { .gradebook { padding: var(--space-3); } }
h1, h2, h3 { font-family: var(--font-display); }
.muted { color: var(--color-ink-muted); }
.filters { display: flex; flex-wrap: wrap; gap: var(--space-4); align-items: end; margin-block: var(--space-4); }
label { display: flex; flex-direction: column; gap: var(--space-2); flex: 1; min-width: min(100%, 180px); }
input, select, button { font: inherit; min-height: var(--touch-min); max-width: 100%; padding: var(--space-2) var(--space-3); border: 1px solid var(--color-border); border-radius: var(--radius-md); color: var(--color-ink); background: var(--color-surface); box-sizing: border-box; }
button { cursor: pointer; } button:disabled { opacity: .6; cursor: not-allowed; }
.actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }
table { width: 100%; border-collapse: collapse; }
th, td { padding: var(--space-4); border-bottom: 1px solid var(--color-border); text-align: left; overflow-wrap: anywhere; }
li { margin-block: var(--space-2); overflow-wrap: anywhere; }
td button { white-space: nowrap; }
.student { margin-top: var(--space-6); padding: var(--space-4); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); background: var(--color-surface); }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
</style>
