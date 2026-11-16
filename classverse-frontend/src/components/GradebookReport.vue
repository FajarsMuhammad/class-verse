<script setup lang="ts">
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
  busy.value = true; error.value = ''
  try { report.value = await gradebookApi.student(orgId.value, user, filter.value) }
  catch (e) { error.value = errorMessage(e) }
  finally { busy.value = false }
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
      <component :is="embedded ? 'h2' : 'h1'" id="gradebook-title">Rapor belajar</component>
      <p class="muted">Nilai kuis adalah nilai akademik 0–100, terpisah dari poin. Riwayat Arena tersedia setelah fitur Arena dirilis.</p>
      <p v-if="error" role="alert">{{ error }} <button type="button" @click="load">Coba lagi</button></p>
      <p v-if="busy" role="status">Memuat rapor…</p>
      <form v-if="staff" class="filters" @submit.prevent="apply">
        <label>Tahun ajaran (roster)
          <select v-model="filter.year_id" @change="changeYear">
            <option :value="undefined">Semua peserta</option>
            <option v-for="y in years" :key="y.id" :value="y.id">{{ y.label }}{{ y.active ? ' · Aktif' : '' }}</option>
          </select>
        </label>
        <label>Kelas pada tahun itu
          <select v-model="filter.class_id" :disabled="!filter.year_id">
            <option :value="undefined">Semua kelas</option>
            <option v-for="c in classes" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </label>
        <label>Gedung (kunci)
          <input v-model.trim="filter.building_key" maxlength="40" placeholder="mis. sains" />
        </label>
        <div class="actions"><button type="submit" :disabled="busy">Terapkan filter</button><button type="button" :disabled="busy" @click="download">Unduh CSV</button></div>
        <p class="muted">Filter kelas memilih siswa yang sedang ditempatkan pada tahun terpilih; hasil lama tidak dipindahkan ke kelas baru.</p>
      </form>
      <table v-if="staff && page">
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
      <nav v-if="staff && page" class="actions" aria-label="Halaman rapor">
        <button type="button" :disabled="busy || offset === 0" @click="move(-1)">Sebelumnya</button>
        <button type="button" :disabled="busy || offset + 20 >= page.total" @click="move(1)">Berikutnya</button>
      </nav>
      <section v-if="report" class="student" aria-live="polite">
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
.gradebook { min-width: 0; }
h1, h2, h3 { font-family: var(--font-display); }
.muted { color: var(--color-ink-muted); }
.filters { display: flex; flex-wrap: wrap; gap: var(--space-4); align-items: end; margin-block: var(--space-4); }
label { display: flex; flex-direction: column; gap: var(--space-2); flex: 1; min-width: min(100%, 180px); }
input, select, button { font: inherit; min-height: 44px; max-width: 100%; padding: var(--space-2) var(--space-3); border: 1px solid var(--color-border); border-radius: var(--radius-md); color: var(--color-ink); background: var(--color-surface); box-sizing: border-box; }
button { cursor: pointer; } button:disabled { opacity: .6; cursor: not-allowed; }
.actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }
table { width: 100%; border-collapse: collapse; display: block; overflow-x: auto; }
th, td { padding: var(--space-2); border-bottom: 1px solid var(--color-border); text-align: left; overflow-wrap: anywhere; }
li { margin-block: var(--space-2); overflow-wrap: anywhere; }
td button { white-space: nowrap; }
.student { margin-top: var(--space-6); padding: var(--space-4); border: 2px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-surface); }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
</style>
