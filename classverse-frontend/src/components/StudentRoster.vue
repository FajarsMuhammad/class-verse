<script setup lang="ts">
import PagedSelect from './PagedSelect.vue'
import RemotePagination from './RemotePagination.vue'
import { PAGE as CLASS_PAGE, hasMore } from '@/api/page'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { classApi, type MoveResult, type Placement, type RosterEntry, type SchoolClass } from '@/api/classes'

// C2c: one server page (max 50) of the roster; search and filter run on the server. Without classId it is the
// Kelola → Siswa page (filter, select many, move); with classId it lists one class (detail → Siswa tab).
// Rights mirror the server: admin moves anyone; a wali kelas adds students without a class to their class and
// removes their own; everyone else reads.
const PAGE = 50
const props = defineProps<{ orgId: string; yearId: string; yearLabel: string; classes: SchoolClass[]; isAdmin: boolean; classId?: string }>()
const emit = defineEmits<{ changed: [] }>()
const items = ref<RosterEntry[]>([])
const total = ref(0)
const unplacedTotal = ref(0)
const offset = ref(0)
const filter = ref('')
const q = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const selected = ref<string[]>([])
const target = ref('')
const candidates = ref<RosterEntry[]>([])
const addID = ref('')
const addQ = ref('')
const historyUser = ref('')
const history = ref<Placement[]>([])
const own = computed(() => props.classes.filter(c => c.my_role === 'homeroom'))
const fixed = computed(() => props.classes.find(c => c.id === props.classId))
const canManageFixed = computed(() => props.isAdmin || fixed.value?.my_role === 'homeroom')
const canMove = computed(() => !props.classId && (props.isAdmin || own.value.length > 0))
const targetNames = ref<Record<string, string>>({})
async function targetOptions(offset: number) {
 const result = await classApi.classes(props.orgId, props.yearId, offset)
 targetNames.value = { ...targetNames.value, ...Object.fromEntries(result.items.map(c => [c.id, c.name])) }
 return { items: [...(props.isAdmin && offset === 0 ? [{ value: 'none', label: 'Keluarkan dari kelas' }] : []), ...result.items.filter(c => props.isAdmin || c.my_role === 'homeroom').map(c => ({ value: c.id, label: c.name }))], more: hasMore(offset, result) }
}
// Class filter chips page on their own (20 per page) so a school with many classes still offers every class.
const chipPage = ref(1)
const chips = ref<SchoolClass[]>([])
const moreChips = ref(false)
let chipGeneration = 0
async function loadChips() {
  if (props.classId) return
  const g = ++chipGeneration, offset = (chipPage.value - 1) * CLASS_PAGE
  try {
    const result = await classApi.classes(props.orgId, props.yearId, offset)
    if (g !== chipGeneration) return
    chips.value = result.items; moreChips.value = hasMore(offset, result)
    targetNames.value = { ...targetNames.value, ...Object.fromEntries(result.items.map(c => [c.id, c.name])) }
  } catch (e) { if (g === chipGeneration) error.value = message(e) }
}
watch(chipPage, loadChips)
const progress = (r: RosterEntry) => r.steps_total ? `${Math.round(((r.steps_done ?? 0) / r.steps_total) * 100)}%` : 'Belum ada tugas'
const className = (id: string | null | undefined) => props.classes.find(c => c.id === id)?.name ?? (id ? targetNames.value[id] ?? 'Kelas terpilih' : 'Belum punya kelas')
const message = (e: unknown) => e instanceof Error ? e.message : 'Perubahan gagal disimpan. Coba lagi.'
let generation = 0
let timer: ReturnType<typeof setTimeout> | undefined
onBeforeUnmount(() => { generation++; clearTimeout(timer) })

async function load() {
  const g = ++generation
  loading.value = true; error.value = ''
  try {
    const classFilter = props.classId ?? filter.value
    const [page, none, cand] = await Promise.all([
      classApi.roster(props.orgId, props.yearId, { class_id: classFilter, q: q.value.trim(), offset: offset.value, limit: PAGE }),
      props.classId ? null : classApi.roster(props.orgId, props.yearId, { class_id: 'none', limit: 1 }),
      props.classId && canManageFixed.value ? classApi.roster(props.orgId, props.yearId, { class_id: 'none', q: addQ.value.trim(), limit: PAGE }) : null,
    ])
    if (g !== generation) return
    items.value = page.items; total.value = page.total
    if (none) unplacedTotal.value = none.total
    if (cand) candidates.value = cand.items
    selected.value = selected.value.filter(id => page.items.some(r => r.user_id === id))
  } catch (e) { if (g === generation) error.value = message(e) }
  finally { if (g === generation) loading.value = false }
}
function reset() { offset.value = 0; load() }
watch(filter, reset)
watch([q, addQ], () => { clearTimeout(timer); timer = setTimeout(reset, 250) })
watch(() => [props.yearId, props.classId], () => { historyUser.value = ''; selected.value = []; filter.value = ''; reset(); if (chipPage.value === 1) void loadChips(); else chipPage.value = 1 })

async function save(action: () => Promise<unknown>) {
  if (saving.value || loading.value) return
  saving.value = true; error.value = ''; notice.value = ''
  try { await action(); emit('changed'); await Promise.all([load(), loadChips()]) }
  catch (e) { error.value = message(e) }
  finally { saving.value = false }
}
function placeTo(student: RosterEntry, to: string | null) {
  if (to === (student.class_id ?? null)) return
  if (!window.confirm(`Ubah kelas ${student.display_name} pada ${props.yearLabel} menjadi ${className(to)}? Riwayat tetap tersimpan.`)) return
  return save(() => classApi.place(props.orgId, props.yearId, student.user_id, to))
}
async function addMember() {
  const student = candidates.value.find(r => r.user_id === addID.value)
  if (student) await placeTo(student, props.classId!)
  addID.value = ''
}
const reason = (r: MoveResult) => r.error === 'has_class' ? 'sudah punya kelas' : 'bukan peserta sekolah'
async function moveSelected() {
  const to = target.value === 'none' ? null : target.value
  const n = selected.value.length
  if (!n || !target.value || !window.confirm(`Pindahkan ${n} siswa ke ${className(to)} pada ${props.yearLabel}? Riwayat tetap tersimpan.`)) return
  const names = new Map(items.value.map(r => [r.user_id, r.display_name]))
  await save(async () => {
    const res = await classApi.move(props.orgId, props.yearId, selected.value, to)
    const failed = res.items.filter(r => r.error)
    notice.value = `${res.items.length - failed.length} siswa dipindahkan.` +
      (failed.length ? ` Tidak dipindahkan: ${failed.map(r => `${names.get(r.user_id) ?? 'siswa'} (${reason(r)})`).join(', ')}.` : '')
    selected.value = []; target.value = ''
  })
}
function page(delta: number) { offset.value = Math.max(0, offset.value + delta * PAGE); load() }
const allOnPage = computed({
  get: () => items.value.length > 0 && items.value.every(r => selected.value.includes(r.user_id)),
  set: v => { selected.value = v ? items.value.map(r => r.user_id) : [] },
})
async function showHistory(student: RosterEntry) {
  if (saving.value || loading.value) return
  saving.value = true; error.value = ''
  try {
    const result = await classApi.history(props.orgId, student.user_id)
    history.value = result.items; historyUser.value = student.display_name
  } catch (e) { error.value = message(e) }
  finally { saving.value = false }
}
function date(value: string) { return new Date(value).toLocaleDateString('id-ID', { timeZone: 'Asia/Jakarta' }) }
onMounted(() => { void load(); void loadChips() })
</script>

<template>
  <div class="class-part" :aria-busy="loading || saving">
    <p v-if="error" role="alert">{{ error }} <button type="button" class="btn-secondary" data-test="retry" :disabled="loading || saving" @click="load">Coba lagi</button></p>
    <p v-if="notice" role="status" data-test="notice">{{ notice }}</p>

    <form v-if="classId && canManageFixed" class="row" data-test="add-member" @submit.prevent="addMember">
      <label>Cari siswa tanpa kelas <input v-model="addQ" type="search" /></label>
      <label>Tambah siswa
        <select v-model="addID" required>
          <option value="" disabled>{{ candidates.length ? 'Pilih siswa' : 'Tidak ada siswa tanpa kelas' }}</option>
          <option v-for="s in candidates" :key="s.user_id" :value="s.user_id">{{ s.display_name }}</option>
        </select>
      </label>
      <button type="submit" class="btn-primary" :disabled="!addID || saving">Tambahkan</button>
    </form>
    <p v-if="classId" class="muted">{{ isAdmin ? 'Siswa dari kelas lain dipindahkan lewat Kelola → Siswa.' : canManageFixed ? 'Siswa yang sudah punya kelas lain dipindahkan oleh admin.' : 'Siswa kelas ini dikelola wali kelas dan admin.' }}</p>

    <div :class="{ 'roster-layout': !classId }">
    <nav v-if="!classId" class="class-filters" aria-label="Filter kelas siswa">
      <button type="button" class="btn-secondary" :aria-pressed="filter === ''" @click="filter = ''">Semua kelas</button>
      <button type="button" class="btn-secondary" data-filter="none" :aria-pressed="filter === 'none'" @click="filter = 'none'">Belum punya kelas ({{ unplacedTotal }})</button>
      <button v-for="c in chips" :key="c.id" type="button" class="btn-secondary" :aria-pressed="filter === c.id" @click="filter = c.id">{{ c.name }} ({{ c.students }})</button>
      <RemotePagination v-if="chipPage > 1 || moreChips" :page="chipPage" :more="moreChips" @move="chipPage += $event" />
    </nav>
    <div class="roster-content">
    <div class="filters">
      <label>Cari siswa <input v-model="q" type="search" data-test="search" /></label>
    </div>

    <div v-if="canMove && selected.length" class="bulk" data-test="bulk">
      <strong>{{ selected.length }} siswa dipilih</strong>
      <PagedSelect v-model="target" test-id="move-target" placeholder="Pilih kelas" label="Pindahkan ke kelas" :load="targetOptions" />
      <button type="button" class="btn-primary" data-action="move" :disabled="!target || saving" @click="moveSelected">Pindahkan</button>
      <button type="button" class="btn-secondary" @click="selected = []">Batal pilih</button>
    </div>

    <div class="pager">
      <label v-if="canMove" class="check"><input v-model="allOnPage" type="checkbox" data-test="select-page" /> Pilih semua di halaman ini</label>
      <span class="muted count">{{ total ? `${offset + 1}–${offset + items.length} dari ${total}` : '0' }} siswa</span>
      <button v-if="offset > 0" type="button" class="btn-secondary" data-action="prev" :disabled="loading" @click="page(-1)">Sebelumnya</button>
      <button v-if="offset + items.length < total" type="button" class="btn-secondary" data-action="next" :disabled="loading" @click="page(1)">Berikutnya</button>
    </div>
    <p v-if="loading" role="status">Memuat…</p>
    <div class="student-table-wrap">
    <table class="student-table" aria-label="Daftar siswa">
      <thead><tr><th v-if="canMove" scope="col"><span class="sr-only">Pilih siswa</span></th><th scope="col">Nama</th><th v-if="!classId" scope="col">Kelas</th><th scope="col">Progres tugas</th><th scope="col"><span class="sr-only">Aksi</span></th></tr></thead>
      <tbody><tr v-for="student in items" :key="student.user_id" :data-student="student.user_id">
        <td v-if="canMove"><label class="check"><input v-model="selected" type="checkbox" :value="student.user_id" :disabled="saving || loading" /><span class="sr-only">Pilih {{ student.display_name }}</span></label></td>
        <th scope="row"><span class="student-initial" aria-hidden="true">{{ student.display_name.slice(0, 1).toLocaleUpperCase('id') }}</span>{{ student.display_name }}</th>
        <td v-if="classId" class="muted">{{ progress(student) }}</td>
        <td v-if="classId" class="student-actions">
          <button v-if="canManageFixed" type="button" class="btn-secondary" :disabled="saving" :aria-label="`Keluarkan ${student.display_name} dari ${fixed?.name}`" @click="placeTo(student, null)">Keluarkan</button>
        </td>
        <template v-else>
          <td><span class="badge">{{ className(student.class_id) }}</span></td>
          <td class="muted">{{ progress(student) }}</td>
          <td class="student-actions"><button type="button" class="btn-secondary" :disabled="saving || loading" :aria-label="`Riwayat kelas ${student.display_name}`" @click="showHistory(student)">Riwayat</button></td>
        </template>
      </tr></tbody>
    </table>
    </div>
    <p v-if="!items.length && !loading" class="muted">{{ classId ? 'Belum ada siswa di kelas ini.' : q || filter ? 'Siswa tidak ditemukan.' : 'Belum ada anggota berperan peserta.' }}</p>
    </div>
    </div>
    <section v-if="historyUser" class="history" aria-live="polite">
      <h3>Riwayat kelas {{ historyUser }}</h3>
      <ol>
        <li v-for="(p, index) in history" :key="index">{{ p.class_name }}, {{ p.year_label }}: {{ date(p.started_at) }} sampai {{ p.ended_at ? date(p.ended_at) : 'sekarang' }}</li>
      </ol>
      <p v-if="!history.length" class="muted">Belum pernah ditempatkan.</p>
      <button type="button" class="btn-secondary" @click="historyUser = ''">Tutup riwayat</button>
    </section>
  </div>
</template>

<style scoped src="../styles/classes.css"></style>
