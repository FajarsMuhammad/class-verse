<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useTemplateRef } from 'vue'
import { classApi, type AcademicYear, type SchoolClass, type RosterEntry, type Placement } from '@/api/classes'

// C2a: Kelas, Siswa and Tahun ajaran are separate Kelola pages sharing this C1 data loader.
// ponytail: roster is loaded whole per year (C1 API); server pagination and bulk move arrive in C2c.
type View = 'classes' | 'students' | 'years'
const props = withDefaults(defineProps<{ orgId: string; isAdmin: boolean; view?: View }>(), { view: 'classes' })
const years = ref<AcademicYear[]>([])
const yearID = ref('')
const classes = ref<SchoolClass[]>([])
const roster = ref<RosterEntry[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const newYear = ref('')
const newClass = ref('')
const creating = ref(false)
const openID = ref('')
const tab = ref<'students' | 'settings'>('students')
const editName = ref('')
const addID = ref('')
const search = ref('')
const classFilter = ref('')
const historyUser = ref('')
const history = ref<Placement[]>([])
const title = useTemplateRef<HTMLElement>('title')
const selectedYear = computed(() => years.value.find(y => y.id === yearID.value))
const opened = computed(() => classes.value.find(c => c.id === openID.value))
const needle = computed(() => search.value.trim().toLocaleLowerCase('id'))
const visibleClasses = computed(() => classes.value.filter(c => c.name.toLocaleLowerCase('id').includes(needle.value)))
const members = computed(() => roster.value.filter(r => r.class_id === openID.value))
const unplaced = computed(() => roster.value.filter(r => !r.class_id))
const visibleRoster = computed(() => roster.value.filter(r =>
  (classFilter.value === '' || (classFilter.value === 'none' ? !r.class_id : r.class_id === classFilter.value)) &&
  r.display_name.toLocaleLowerCase('id').includes(needle.value)))
const className = (id: string | null | undefined) => classes.value.find(c => c.id === id)?.name ?? 'Belum punya kelas'
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
    if (!yearID.value || props.view === 'years') {
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
  if (openID.value && !c.items.some(x => x.id === openID.value)) openID.value = ''
}
async function changeYear(event: Event) {
  yearID.value = (event.target as HTMLSelectElement).value
  const g = ++generation
  loading.value = true; error.value = ''; historyUser.value = ''; openID.value = ''; classFilter.value = ''
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
async function activate(y: AcademicYear) {
  if (!window.confirm(`Jadikan ${y.label} tahun ajaran aktif? Siswa tidak dipindahkan otomatis.`)) return
  await save(() => classApi.activate(props.orgId, y.id))
}
async function createYear() {
  await save(async () => { await classApi.createYear(props.orgId, newYear.value); newYear.value = '' })
}
async function createClass() {
  let created: SchoolClass | undefined
  await save(async () => {
    created = await classApi.create(props.orgId, yearID.value, newClass.value)
    newClass.value = ''; creating.value = false
  })
  // Open only after the reload so the detail renders from server truth and can take focus.
  const c = created && classes.value.find(x => x.id === created!.id)
  if (c) await openClass(c)
}
async function openClass(c: SchoolClass) {
  openID.value = c.id; tab.value = 'students'; editName.value = c.name; addID.value = ''
  await nextTick()
  title.value?.focus()
}
async function rename() {
  await save(() => classApi.rename(props.orgId, openID.value, editName.value))
}
async function archive() {
  const c = opened.value
  if (!c || !window.confirm(`Arsipkan kelas ${c.name}? Riwayat penempatan tetap tersimpan.`)) return
  await save(async () => { await classApi.archive(props.orgId, c.id); openID.value = '' })
}
function placeTo(student: RosterEntry, target: string | null) {
  if (target === (student.class_id ?? null)) return
  if (!window.confirm(`Ubah kelas ${student.display_name} pada ${selectedYear.value?.label} menjadi ${className(target)}? Riwayat tetap tersimpan.`)) return
  return save(() => classApi.place(props.orgId, yearID.value, student.user_id, target))
}
async function place(student: RosterEntry, event: Event) {
  const select = event.target as HTMLSelectElement
  const target = select.value || null
  // Keep server truth visible on cancel/error; the new value is shown only after a successful reload.
  select.value = student.class_id ?? ''
  await placeTo(student, target)
}
async function addMember() {
  const student = roster.value.find(r => r.user_id === addID.value)
  if (student) await placeTo(student, openID.value)
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
  <section class="class-panel" :aria-busy="loading || saving" :aria-label="view === 'years' ? 'Tahun ajaran' : view === 'students' ? 'Siswa' : 'Kelas'">
    <p v-if="error" role="alert">{{ error }} <button type="button" class="btn-secondary" data-test="retry" :disabled="loading || saving" @click="load">Coba lagi</button></p>
    <p v-if="loading" role="status">Memuat…</p>
    <fieldset :disabled="loading || saving">
      <legend class="sr-only">Pengelolaan kelas</legend>

      <template v-if="view === 'years'">
        <p class="muted">Tahun aktif dipilih admin, bukan berdasarkan kalender. Mengganti tahun aktif tidak memindahkan siswa.</p>
        <form v-if="isAdmin" data-test="create-year" class="row" @submit.prevent="createYear">
          <label>Tahun ajaran baru <input v-model="newYear" required placeholder="2027/2028" pattern="[0-9]{4}/[0-9]{4}" maxlength="9" /></label>
          <button type="submit" class="btn-primary">Tambah</button>
        </form>
        <ul class="list">
          <li v-for="y in years" :key="y.id" :data-year="y.id">
            <div><strong>{{ y.label }}</strong></div>
            <span v-if="y.active" class="badge">Aktif</span>
            <button v-else-if="isAdmin" type="button" class="btn-secondary" :aria-label="`Jadikan ${y.label} aktif`" @click="activate(y)">Jadikan aktif</button>
          </li>
        </ul>
        <p v-if="!years.length && !loading" class="muted">Belum ada tahun ajaran.</p>
      </template>

      <p v-else-if="!years.length && !loading" class="muted">
        Belum ada tahun ajaran. {{ isAdmin ? 'Tambahkan di Kelola → Tahun ajaran.' : 'Minta admin sekolah menambahkan tahun ajaran.' }}
      </p>

      <template v-else-if="view === 'classes' && opened">
        <div class="detail-head">
          <button type="button" class="btn-secondary" data-action="back" @click="openID = ''">Kembali ke daftar</button>
          <h3 ref="title" class="detail-title" tabindex="-1">{{ opened.name }}</h3>
          <span class="muted">{{ selectedYear?.label }} · {{ members.length }} siswa</span>
        </div>
        <div class="tabs" role="tablist" aria-label="Detail kelas">
          <button type="button" role="tab" :aria-selected="tab === 'students'" data-tab="students" @click="tab = 'students'">Siswa</button>
          <button type="button" role="tab" :aria-selected="tab === 'settings'" data-tab="settings" @click="tab = 'settings'">Pengaturan</button>
        </div>
        <div v-if="tab === 'students'" role="tabpanel" aria-label="Siswa">
          <form class="row" data-test="add-member" @submit.prevent="addMember">
            <label>Tambah siswa tanpa kelas
              <select v-model="addID" required>
                <option value="" disabled>{{ unplaced.length ? 'Pilih siswa' : 'Semua siswa sudah punya kelas' }}</option>
                <option v-for="s in unplaced" :key="s.user_id" :value="s.user_id">{{ s.display_name }}</option>
              </select>
            </label>
            <button type="submit" class="btn-primary" :disabled="!addID">Tambahkan</button>
          </form>
          <p class="muted">Siswa dari kelas lain dipindahkan lewat Kelola → Siswa.</p>
          <ul class="list">
            <li v-for="s in members" :key="s.user_id" :data-member="s.user_id">
              <div><strong>{{ s.display_name }}</strong></div>
              <button type="button" class="btn-secondary" :aria-label="`Keluarkan ${s.display_name} dari ${opened.name}`" @click="placeTo(s, null)">Keluarkan</button>
            </li>
          </ul>
          <p v-if="!members.length" class="muted">Belum ada siswa di kelas ini.</p>
        </div>
        <div v-else role="tabpanel" aria-label="Pengaturan" class="settings">
          <form class="row" @submit.prevent="rename">
            <label>Nama kelas <input v-model="editName" required maxlength="40" /></label>
            <button type="submit" class="btn-primary" :disabled="editName.trim() === opened.name">Simpan nama</button>
          </form>
          <div class="danger">
            <p class="muted">{{ opened.students ? 'Keluarkan semua siswa sebelum mengarsipkan kelas.' : 'Kelas arsip tidak tampil lagi; riwayat penempatan tetap tersimpan.' }}</p>
            <button type="button" class="btn-secondary" data-action="archive" :disabled="opened.students > 0" @click="archive">Arsipkan kelas</button>
          </div>
        </div>
      </template>

      <template v-else>
        <div class="filters">
          <label>Tahun ajaran
            <select data-test="year" :value="yearID" @change="changeYear">
              <option v-for="y in years" :key="y.id" :value="y.id">{{ y.label }}{{ y.active ? ' (aktif)' : '' }}</option>
            </select>
          </label>
          <label v-if="view === 'students'">Kelas
            <select v-model="classFilter" data-test="class-filter">
              <option value="">Semua kelas</option>
              <option value="none">Belum punya kelas ({{ unplaced.length }})</option>
              <option v-for="c in classes" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
          </label>
          <label>{{ view === 'students' ? 'Cari siswa' : 'Cari kelas' }} <input v-model="search" type="search" /></label>
          <button v-if="view === 'classes' && !creating" type="button" class="btn-primary new" @click="creating = true">+ Kelas baru</button>
        </div>
        <form v-if="view === 'classes' && creating" class="row create" @submit.prevent="createClass">
          <label>Nama kelas baru <input v-model="newClass" required maxlength="40" placeholder="XI IPA 1" /></label>
          <button type="submit" class="btn-primary">Buat kelas</button>
          <button type="button" class="btn-secondary" @click="creating = false; newClass = ''">Batal</button>
        </form>

        <template v-if="view === 'classes'">
          <ul class="list">
            <li v-for="c in visibleClasses" :key="c.id" :data-class="c.id">
              <div><strong>{{ c.name }}</strong><p class="meta">{{ c.students }} siswa</p></div>
              <button type="button" class="btn-secondary" :aria-label="`Buka ${c.name}`" @click="openClass(c)">Buka</button>
            </li>
          </ul>
          <p v-if="!visibleClasses.length && !loading" class="muted">{{ classes.length ? 'Kelas tidak ditemukan.' : 'Belum ada kelas pada tahun ajaran ini.' }}</p>
        </template>

        <template v-else>
          <p class="muted count">{{ visibleRoster.length }} dari {{ roster.length }} siswa</p>
          <ul class="list">
            <li v-for="student in visibleRoster" :key="student.user_id" :data-student="student.user_id">
              <div><strong>{{ student.display_name }}</strong></div>
              <label class="inline"><span class="sr-only">Kelas {{ student.display_name }}</span>
                <select :value="student.class_id ?? ''" @change="place(student, $event)">
                  <option value="">Belum punya kelas</option>
                  <option v-for="c in classes" :key="c.id" :value="c.id">{{ c.name }}</option>
                </select>
              </label>
              <button type="button" class="btn-secondary" :aria-label="`Riwayat kelas ${student.display_name}`" @click="showHistory(student)">Riwayat</button>
            </li>
          </ul>
          <p v-if="!visibleRoster.length && !loading" class="muted">{{ roster.length ? 'Siswa tidak ditemukan.' : 'Belum ada anggota berperan peserta.' }}</p>
          <section v-if="historyUser" class="history" aria-live="polite">
            <h3>Riwayat kelas {{ historyUser }}</h3>
            <ol>
              <li v-for="(p, index) in history" :key="index">{{ p.class_name }}, {{ p.year_label }}: {{ date(p.started_at) }} sampai {{ p.ended_at ? date(p.ended_at) : 'sekarang' }}</li>
            </ol>
            <p v-if="!history.length" class="muted">Belum pernah ditempatkan.</p>
            <button type="button" class="btn-secondary" @click="historyUser = ''">Tutup riwayat</button>
          </section>
        </template>
      </template>
    </fieldset>
  </section>
</template>

<style scoped>
.class-panel { display: grid; gap: var(--space-3); }
fieldset { border: 0; padding: 0; margin: 0; min-width: 0; display: grid; gap: var(--space-3); }
.muted, .meta { color: var(--kv-text-muted); font-size: var(--text-sm); margin: 0; }
label { display: grid; gap: var(--space-1); min-width: 0; font-size: var(--text-sm); font-weight: var(--weight-semibold); }
.row, .filters { display: flex; flex-wrap: wrap; gap: var(--space-3); align-items: end; }
.filters label { width: min(100%, 220px); }
.filters .new { margin-left: auto; }
.create { padding: var(--space-3); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); }
input, select { box-sizing: border-box; max-width: 100%; min-height: var(--touch-min); padding: var(--space-2) var(--space-3); border: 1px solid var(--color-input-border); border-radius: var(--control-radius); background: var(--kv-surface); color: var(--kv-ink); font: inherit; font-weight: var(--weight-regular); }
.list { list-style: none; margin: 0; padding: 0; }
.list > li { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-3); padding: var(--space-3) 0; border-bottom: 1px solid var(--kv-line); }
.list > li > div { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.list strong { font-size: var(--text-md); font-weight: var(--weight-semibold); }
.inline { width: min(100%, 200px); }
.badge { padding: 2px var(--space-2); border-radius: var(--radius-pill, 999px); background: var(--color-primary-soft); color: var(--color-primary-strong); font-size: var(--text-xs); font-weight: var(--weight-semibold); }
.detail-head { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-3); }
.detail-title { margin: 0; font: var(--weight-semibold) var(--text-lg)/var(--line-tight) var(--font-display); }
.detail-title:focus { outline: none; }
.tabs { display: flex; gap: var(--space-1); border-bottom: 1px solid var(--kv-line); }
.tabs button { min-height: var(--touch-min); padding: 0 var(--space-3); border: 0; border-bottom: 2px solid transparent; background: none; color: var(--kv-text-muted); font: var(--weight-semibold) var(--text-sm)/1 var(--font-body); cursor: pointer; }
.tabs button[aria-selected='true'] { color: var(--color-primary-strong); border-bottom-color: var(--color-primary-strong); }
.tabs button:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: -3px; }
[role='tabpanel'] { display: grid; gap: var(--space-3); }
.settings form label { width: min(100%, 320px); }
.danger { display: grid; gap: var(--space-2); justify-items: start; padding-top: var(--space-3); border-top: 1px solid var(--kv-line); }
.history { padding: var(--space-3); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); }
.history h3 { margin: 0; font-size: var(--text-md); }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
</style>
