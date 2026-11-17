<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useTemplateRef } from 'vue'
import { classApi, type AcademicYear, type SchoolClass } from '@/api/classes'
import ClassTeachers from './ClassTeachers.vue'
import StudentRoster from './StudentRoster.vue'
import YearSetup from './YearSetup.vue'

// C2: Kelas, Siswa and Tahun ajaran are separate Kelola pages. Admin manages years, classes, class teachers and
// placements; a wali kelas manages students of their own class; everyone else reads (server enforces it too).
type View = 'classes' | 'students' | 'years'
const props = withDefaults(defineProps<{ orgId: string; isAdmin: boolean; view?: View }>(), { view: 'classes' })
const years = ref<AcademicYear[]>([])
const yearID = ref('')
const classes = ref<SchoolClass[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const newYear = ref('')
const newClass = ref('')
const creating = ref(false)
const openID = ref('')
const tab = ref<'students' | 'teachers' | 'settings'>('students')
const editName = ref('')
const search = ref('')
const title = useTemplateRef<HTMLElement>('title')
const selectedYear = computed(() => years.value.find(y => y.id === yearID.value))
const opened = computed(() => classes.value.find(c => c.id === openID.value))
const needle = computed(() => search.value.trim().toLocaleLowerCase('id'))
const visibleClasses = computed(() => classes.value.filter(c => c.name.toLocaleLowerCase('id').includes(needle.value)))
const roleLabel = (c: SchoolClass) => c.my_role === 'homeroom' ? 'Wali kelas Anda' : c.my_role === 'subject' ? 'Anda guru mapel' : ''
let generation = 0
onBeforeUnmount(() => { generation++ })
const message = (e: unknown) => e instanceof Error ? e.message : 'Perubahan gagal disimpan. Coba lagi.'

async function load() {
  const g = ++generation
  loading.value = true
  error.value = ''
  try {
    const result = await classApi.years(props.orgId)
    if (g !== generation) return
    years.value = result.items
    yearID.value = result.items.find(y => y.id === yearID.value)?.id ?? result.items.find(y => y.active)?.id ?? result.items[0]?.id ?? ''
    if (!yearID.value || props.view === 'years') {
      classes.value = []
      return
    }
    await fetchYear(g)
  } catch (e) { if (g === generation) error.value = message(e) }
  finally { if (g === generation) loading.value = false }
}
async function fetchYear(g: number) {
  const c = await classApi.classes(props.orgId, yearID.value)
  if (g !== generation) return
  classes.value = c.items
  if (openID.value && !c.items.some(x => x.id === openID.value)) openID.value = ''
}
async function refreshClasses() {
  const g = generation
  try { await fetchYear(g) } catch (e) { if (g === generation) error.value = message(e) }
}
async function changeYear(event: Event) {
  yearID.value = (event.target as HTMLSelectElement).value
  const g = ++generation
  loading.value = true; error.value = ''; openID.value = ''
  classes.value = []
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
  openID.value = c.id; tab.value = 'students'; editName.value = c.name
  await nextTick()
  title.value?.focus()
}
async function rename() {
  await save(() => classApi.rename(props.orgId, openID.value, editName.value))
}
async function archive() {
  const c = opened.value
  if (!c || !window.confirm(`Arsipkan kelas ${c.name}? Pengajar kelas dilepas; riwayat penempatan tetap tersimpan.`)) return
  await save(async () => { await classApi.archive(props.orgId, c.id); openID.value = '' })
}
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
          <span class="muted">{{ selectedYear?.label }} · {{ opened.students }} siswa · Wali kelas: {{ opened.homeroom?.name ?? 'belum ada' }}</span>
        </div>
        <div class="tabs" role="tablist" aria-label="Detail kelas">
          <button type="button" role="tab" :aria-selected="tab === 'students'" data-tab="students" @click="tab = 'students'">Siswa</button>
          <button type="button" role="tab" :aria-selected="tab === 'teachers'" data-tab="teachers" @click="tab = 'teachers'">Pengajar</button>
          <button v-if="isAdmin" type="button" role="tab" :aria-selected="tab === 'settings'" data-tab="settings" @click="tab = 'settings'">Pengaturan</button>
        </div>
        <div v-if="tab === 'students'" role="tabpanel" aria-label="Siswa">
          <StudentRoster :org-id="orgId" :year-id="yearID" :year-label="selectedYear?.label ?? ''" :classes="classes" :is-admin="isAdmin" :class-id="opened.id" @changed="refreshClasses" />
        </div>
        <div v-else-if="tab === 'teachers'" role="tabpanel" aria-label="Pengajar">
          <ClassTeachers :org-id="orgId" :class-id="opened.id" :is-admin="isAdmin" @changed="refreshClasses" />
        </div>
        <div v-else role="tabpanel" aria-label="Pengaturan" class="settings">
          <form class="row" @submit.prevent="rename">
            <label>Nama kelas <input v-model="editName" required maxlength="40" /></label>
            <button type="submit" class="btn-primary" :disabled="editName.trim() === opened.name">Simpan nama</button>
          </form>
          <div class="danger">
            <p class="muted">{{ opened.students ? 'Keluarkan semua siswa sebelum mengarsipkan kelas.' : 'Kelas arsip tidak tampil lagi; pengajar kelas dilepas dan riwayat penempatan tetap tersimpan.' }}</p>
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
          <label v-if="view === 'classes'">Cari kelas <input v-model="search" type="search" /></label>
          <button v-if="view === 'classes' && isAdmin && !creating" type="button" class="btn-primary new" @click="creating = true">+ Kelas baru</button>
        </div>
        <form v-if="view === 'classes' && creating" class="row create" @submit.prevent="createClass">
          <label>Nama kelas baru <input v-model="newClass" required maxlength="40" placeholder="XI IPA 1" /></label>
          <button type="submit" class="btn-primary">Buat kelas</button>
          <button type="button" class="btn-secondary" @click="creating = false; newClass = ''">Batal</button>
        </form>

        <template v-if="view === 'classes'">
          <ul class="list">
            <li v-for="c in visibleClasses" :key="c.id" :data-class="c.id">
              <div>
                <strong>{{ c.name }}</strong>
                <p class="meta">{{ c.students }} siswa · Wali kelas: {{ c.homeroom?.name ?? 'belum ada' }} · {{ c.subject_teachers }} guru mapel</p>
              </div>
              <span v-if="roleLabel(c)" class="badge">{{ roleLabel(c) }}</span>
              <button type="button" class="btn-secondary" :aria-label="`Buka ${c.name}`" @click="openClass(c)">Buka</button>
            </li>
          </ul>
          <p v-if="!visibleClasses.length && !loading" class="muted">{{ classes.length ? 'Kelas tidak ditemukan.' : isAdmin ? 'Belum ada kelas pada tahun ajaran ini.' : 'Belum ada kelas pada tahun ajaran ini. Kelas dibuat admin sekolah.' }}</p>
        </template>

        <StudentRoster v-else-if="yearID" :key="yearID" :org-id="orgId" :year-id="yearID" :year-label="selectedYear?.label ?? ''" :classes="classes" :is-admin="isAdmin" @changed="refreshClasses" />
      </template>
    </fieldset>
    <YearSetup v-if="view === 'years' && isAdmin && years.length > 1" :org-id="orgId" :years="years" />
  </section>
</template>

<style scoped src="../styles/classes.css"></style>
