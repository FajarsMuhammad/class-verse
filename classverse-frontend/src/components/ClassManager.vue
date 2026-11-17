<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { classApi, type AcademicYear, type SchoolClass, type RosterEntry, type Placement } from '@/api/classes'

const props = defineProps<{ orgId: string; isAdmin: boolean }>()
const years = ref<AcademicYear[]>([])
const yearID = ref('')
const classes = ref<SchoolClass[]>([])
const roster = ref<RosterEntry[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const newYear = ref('')
const newClass = ref('')
const editID = ref('')
const editName = ref('')
const search = ref('')
const historyUser = ref('')
const history = ref<Placement[]>([])
const selectedYear = computed(() => years.value.find(y => y.id === yearID.value))
const visibleRoster = computed(() => roster.value.filter(r => r.display_name.toLocaleLowerCase('id').includes(search.value.toLocaleLowerCase('id'))))
let generation = 0
onBeforeUnmount(() => { generation++ })
const message = (e: unknown) => e instanceof Error ? e.message : 'Perubahan gagal disimpan. Coba lagi.'

async function load() {
  const g = ++generation
  loading.value = true
  error.value = ''
  historyUser.value = ''
  try {
    const result = await classApi.years(props.orgId)
    if (g !== generation) return
    years.value = result.items
    yearID.value = result.items.find(y => y.id === yearID.value)?.id ?? result.items.find(y => y.active)?.id ?? result.items[0]?.id ?? ''
    if (!yearID.value) {
      classes.value = []; roster.value = []
      return
    }
    await fetchYear(g)
  } catch (e) { if (g === generation) error.value = message(e) }
  finally { if (g === generation) loading.value = false }
}
async function fetchYear(g: number) {
  const [c, r] = await Promise.all([classApi.classes(props.orgId, yearID.value), classApi.roster(props.orgId, yearID.value)])
  if (g !== generation) return
  classes.value = c.items
  roster.value = r.items
}
async function changeYear(event: Event) {
  yearID.value = (event.target as HTMLSelectElement).value
  const g = ++generation
  loading.value = true; error.value = ''; historyUser.value = ''; editID.value = ''
  classes.value = []; roster.value = []
  try { await fetchYear(g) }
  catch (e) { if (g === generation) error.value = message(e) }
  finally { if (g === generation) loading.value = false }
}
async function save(action: () => Promise<unknown>) {
  if (saving.value || loading.value) return
  saving.value = true; error.value = ''
  const g = generation
  try {
    await action()
    if (g === generation) await load()
  } catch (e) { if (g === generation) error.value = message(e) }
  finally { saving.value = false }
}
async function activateYear() { await save(() => classApi.activate(props.orgId, yearID.value)) }
async function createYear() {
  await save(async () => {
    const y = await classApi.createYear(props.orgId, newYear.value)
    yearID.value = y.id; newYear.value = ''
  })
}
async function createClass() {
  await save(async () => { await classApi.create(props.orgId, yearID.value, newClass.value); newClass.value = '' })
}
async function rename() {
  await save(async () => { await classApi.rename(props.orgId, editID.value, editName.value); editID.value = '' })
}
async function archive(c: SchoolClass) {
  if (!window.confirm(`Arsipkan kelas ${c.name}? Riwayat penempatan tetap tersimpan.`)) return
  await save(() => classApi.archive(props.orgId, c.id))
}
async function place(student: RosterEntry, event: Event) {
  const select = event.target as HTMLSelectElement
  const target = select.value || null
  // Keep server truth visible on cancel/error; the new value is shown only after a successful reload.
  select.value = student.class_id ?? ''
  if (target === student.class_id) return
  const name = classes.value.find(c => c.id === target)?.name ?? 'Belum ditempatkan'
  if (!window.confirm(`Ubah penempatan ${student.display_name} pada ${selectedYear.value?.label} menjadi ${name}? Riwayat tetap tersimpan.`)) return
  await save(() => classApi.place(props.orgId, yearID.value, student.user_id, target))
}
async function showHistory(student: RosterEntry) {
  if (saving.value || loading.value) return
  saving.value = true; error.value = ''
  const g = generation
  try {
    const result = await classApi.history(props.orgId, student.user_id)
    if (g === generation) { history.value = result.items; historyUser.value = student.display_name }
  } catch (e) { if (g === generation) error.value = message(e) }
  finally { saving.value = false }
}
function date(value: string) { return new Date(value).toLocaleDateString('id-ID', { timeZone: 'Asia/Jakarta' }) }
onMounted(load)
</script>

<template>
  <section class="class-panel" aria-labelledby="class-heading" :aria-busy="loading || saving">
    <h2 id="class-heading">Kelas &amp; tahun ajaran</h2>
    <p class="muted">Rombel berbeda dari ruang virtual. Pindah kelas tidak menghapus hasil belajar. Tidak ada kenaikan kelas otomatis.</p>
    <p v-if="error" role="alert">{{ error }} <button type="button" data-test="retry" :disabled="loading || saving" @click="load">Coba lagi</button></p>
    <p v-if="loading" role="status">Memuat kelas…</p>
    <fieldset :disabled="loading || saving">
      <legend class="sr-only">Pengelolaan kelas</legend>
      <form v-if="isAdmin" data-test="create-year" class="row" @submit.prevent="createYear">
        <label>Tahun ajaran baru <input v-model="newYear" required placeholder="2026/2027" pattern="[0-9]{4}/[0-9]{4}" maxlength="9" /></label>
        <button type="submit">Tambah tahun ajaran</button>
      </form>
      <template v-if="years.length">
        <div class="row">
          <label>Tahun ajaran
            <select data-test="year" :value="yearID" @change="changeYear">
              <option v-for="y in years" :key="y.id" :value="y.id">{{ y.label }}{{ y.active ? ' · Aktif' : '' }}</option>
            </select>
          </label>
          <button v-if="isAdmin && selectedYear && !selectedYear.active" type="button" data-test="activate-year" @click="activateYear">Jadikan tahun aktif</button>
        </div>
        <p class="muted">Tahun aktif dipilih admin sekolah, bukan berdasarkan kalender. Tahun lama tetap tersimpan.</p>
        <h3>Kelas {{ selectedYear?.label }}</h3>
        <form class="row" @submit.prevent="createClass">
          <label>Nama kelas baru <input v-model="newClass" required maxlength="40" placeholder="XI IPA 1" /></label>
          <button type="submit">Tambah kelas</button>
        </form>
        <ul class="classes">
          <li v-for="c in classes" :key="c.id">
            <form v-if="editID === c.id" class="row" @submit.prevent="rename">
              <label>Nama kelas <input v-model="editName" required maxlength="40" /></label>
              <button type="submit">Simpan nama</button>
              <button type="button" @click="editID = ''">Batal</button>
            </form>
            <template v-else>
              <span><strong>{{ c.name }}</strong> · {{ c.students }} siswa</span>
              <div class="actions">
                <button type="button" :aria-label="`Ubah nama ${c.name}`" @click="editID = c.id; editName = c.name">Ubah nama</button>
                <button type="button" :disabled="c.students > 0" :aria-label="`Arsipkan ${c.name}`" @click="archive(c)">Arsipkan</button>
              </div>
            </template>
          </li>
        </ul>
        <p v-if="!classes.length && !loading">Belum ada kelas pada tahun ajaran ini.</p>
        <p class="muted">Kelas hanya bisa diarsipkan setelah semua siswa dipindahkan atau dikeluarkan.</p>
        <h3>Siswa sekolah</h3>
        <label>Cari nama siswa <input v-model="search" type="search" /></label>
        <ul class="students">
          <li v-for="student in visibleRoster" :key="student.user_id" :data-student="student.user_id">
            <strong>{{ student.display_name }}</strong>
            <label>Kelas {{ student.display_name }}
              <select :value="student.class_id ?? ''" @change="place(student, $event)">
                <option value="">Belum ditempatkan</option>
                <option v-for="c in classes" :key="c.id" :value="c.id">{{ c.name }}</option>
              </select>
            </label>
            <button type="button" :aria-label="`Riwayat kelas ${student.display_name}`" @click="showHistory(student)">Riwayat</button>
          </li>
        </ul>
        <p v-if="!visibleRoster.length && !loading">{{ roster.length ? 'Nama siswa tidak ditemukan.' : 'Belum ada anggota berperan peserta.' }}</p>
      </template>
      <p v-else-if="!loading">Belum ada tahun ajaran. {{ isAdmin ? 'Tambahkan tahun ajaran pertama untuk mulai.' : 'Minta admin sekolah menambahkan tahun ajaran.' }}</p>
      <section v-if="historyUser" aria-live="polite">
        <h3>Riwayat kelas {{ historyUser }}</h3>
        <ol>
          <li v-for="(p, index) in history" :key="index">{{ p.class_name }} · {{ p.year_label }} — {{ date(p.started_at) }} sampai {{ p.ended_at ? date(p.ended_at) : 'sekarang' }}</li>
        </ol>
        <p v-if="!history.length">Belum pernah ditempatkan.</p>
        <button type="button" @click="historyUser = ''">Tutup riwayat</button>
      </section>
    </fieldset>
  </section>
</template>

<style scoped>
.class-panel { padding: var(--space-6); margin-top: var(--space-6); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); background: var(--color-surface); }
h2, h3 { font-family: var(--font-display); }
h2 { margin-top: 0; }
.muted { color: var(--color-ink-muted); }
fieldset { border: 0; padding: 0; margin: 0; min-width: 0; }
label { display: flex; flex-direction: column; gap: var(--space-2); min-width: 0; }
.row, .actions { display: flex; flex-wrap: wrap; gap: var(--space-3); align-items: end; }
.row { margin-block: var(--space-4); }
input, select, button { font: inherit; min-height: var(--touch-min); border: 1px solid var(--color-ink); border-radius: var(--radius-md); padding: var(--space-2) var(--space-3); max-width: 100%; box-sizing: border-box; }
input, select { background: var(--color-surface); color: var(--color-ink); }
button { cursor: pointer; color: var(--color-ink); background: var(--color-surface); }
button:disabled { opacity: .6; cursor: not-allowed; }
.classes, .students { padding: 0; list-style: none; }
.classes > li, .students > li { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-3); padding-block: var(--space-3); border-bottom: 1px solid var(--color-ink-muted); overflow-wrap: anywhere; }
.classes > li > span, .students > li > strong { flex: 1; min-width: 0; }
.students label { flex: 1; min-width: min(100%, 180px); }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
</style>
