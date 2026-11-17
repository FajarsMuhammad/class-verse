<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { classApi, type AcademicYear, type Person, type SchoolClass } from '@/api/classes'

// C2d (admin): prepare a new year. 1) copy class structure (optionally teachers), students never copied;
// 2) naik kelas: explicit old → new class mapping, preview, confirm. Nothing is promoted automatically.
const props = defineProps<{ orgId: string; years: AcademicYear[] }>()
const emit = defineEmits<{ changed: [] }>()
const toID = ref('')
const fromID = ref('')
const withTeachers = ref(true)
const fromClasses = ref<SchoolClass[]>([])
const toClasses = ref<SchoolClass[]>([])
const mapping = ref<Record<string, string>>({})
const step = ref<'map' | 'preview'>('map')
const busy = ref(false)
const error = ref('')
const notice = ref('')
const skipped = ref<Person[]>([])
const label = (id: string) => props.years.find(y => y.id === id)?.label ?? ''
const sources = computed(() => props.years.filter(y => y.id !== toID.value))
const chosen = computed(() => fromClasses.value.filter(c => mapping.value[c.id]))
const moving = computed(() => chosen.value.reduce((n, c) => n + c.students, 0))
const message = (e: unknown) => e instanceof Error ? e.message : 'Perubahan gagal disimpan. Coba lagi.'
let generation = 0

watch(() => props.years, ys => {
  if (!ys.some(y => y.id === toID.value)) toID.value = ys[0]?.id ?? ''
}, { immediate: true })
watch(toID, () => { if (fromID.value === toID.value) fromID.value = '' })
watch([toID, fromID], loadClasses)

async function loadClasses() {
  const g = ++generation
  step.value = 'map'; error.value = ''; notice.value = ''; skipped.value = []
  fromClasses.value = []; toClasses.value = []; mapping.value = {}
  if (!toID.value || !fromID.value) return
  busy.value = true
  try {
    const [from, to] = await Promise.all([classApi.classes(props.orgId, fromID.value), classApi.classes(props.orgId, toID.value)])
    if (g !== generation) return
    fromClasses.value = from.items; toClasses.value = to.items
  } catch (e) { if (g === generation) error.value = message(e) }
  finally { if (g === generation) busy.value = false }
}
async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true; error.value = ''; notice.value = ''; skipped.value = []
  try { await action(); emit('changed') }
  catch (e) { error.value = message(e) }
  finally { busy.value = false }
}
async function copy() {
  if (!window.confirm(`Salin struktur kelas ${label(fromID.value)} ke ${label(toID.value)}${withTeachers.value ? ' beserta pengajarnya' : ''}? Siswa tidak ikut disalin.`)) return
  await run(async () => {
    const res = await classApi.copy(props.orgId, toID.value, fromID.value, withTeachers.value)
    const to = await classApi.classes(props.orgId, toID.value)
    toClasses.value = to.items
    notice.value = `${res.created} kelas dibuat.` + (res.skipped.length ? ` Sudah ada, dilewati: ${res.skipped.join(', ')}.` : '')
  })
}
async function promote() {
  await run(async () => {
    const res = await classApi.promote(props.orgId, toID.value, fromID.value,
      chosen.value.map(c => ({ from_class_id: c.id, to_class_id: mapping.value[c.id]! })))
    skipped.value = res.skipped
    notice.value = `${res.moved} siswa ditempatkan di ${label(toID.value)}.` + (res.skipped.length ? ` ${res.skipped.length} siswa dilewati karena sudah punya kelas atau bukan peserta lagi:` : '')
    step.value = 'map'; mapping.value = {}
    toClasses.value = (await classApi.classes(props.orgId, toID.value)).items
  })
}
const nameIn = (list: SchoolClass[], id: string | undefined) => list.find(c => c.id === id)?.name ?? ''
</script>

<template>
  <section class="class-part" data-test="year-setup" :aria-busy="busy">
    <h3 class="section-title">Siapkan tahun ajaran</h3>
    <p class="muted">Salin struktur kelas dari tahun lain, lalu naikkan siswa dengan memetakan kelas lama ke kelas baru. Tidak ada yang dipindah otomatis.</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="notice" role="status" data-test="setup-notice">{{ notice }}</p>
    <ul v-if="skipped.length" class="skipped">
      <li v-for="p in skipped" :key="p.id">{{ p.name }}</li>
    </ul>
    <fieldset :disabled="busy" class="class-part">
      <legend class="sr-only">Siapkan tahun ajaran</legend>
      <div class="filters">
        <label>Tahun tujuan
          <select v-model="toID" data-test="to-year">
            <option v-for="y in years" :key="y.id" :value="y.id">{{ y.label }}</option>
          </select>
        </label>
        <label>Tahun sumber
          <select v-model="fromID" data-test="from-year">
            <option value="" disabled>Pilih tahun</option>
            <option v-for="y in sources" :key="y.id" :value="y.id">{{ y.label }}</option>
          </select>
        </label>
      </div>
      <p v-if="sources.length === 0" class="muted">Tambahkan tahun ajaran lain dulu.</p>
      <template v-if="fromID">
        <h4 class="section-title">1. Salin struktur kelas</h4>
        <div class="row">
          <label class="check"><input v-model="withTeachers" type="checkbox" /> Sertakan wali kelas dan guru mapel</label>
          <button type="button" class="btn-primary" data-action="copy" :disabled="!fromClasses.length" @click="copy">Salin {{ fromClasses.length }} kelas</button>
        </div>
        <p v-if="!fromClasses.length && !busy" class="muted">Tahun sumber belum punya kelas.</p>

        <h4 class="section-title">2. Naik kelas</h4>
        <template v-if="step === 'map'">
          <p v-if="!toClasses.length && !busy" class="muted">Tahun tujuan belum punya kelas. Salin struktur atau buat kelas dulu.</p>
          <ul v-else class="list">
            <li v-for="c in fromClasses" :key="c.id" :data-from="c.id">
              <div><strong>{{ c.name }}</strong><p class="meta">{{ c.students }} siswa</p></div>
              <label class="inline"><span class="sr-only">Kelas tujuan untuk {{ c.name }}</span>
                <select v-model="mapping[c.id]">
                  <option :value="undefined">Lewati</option>
                  <option v-for="t in toClasses" :key="t.id" :value="t.id">{{ t.name }}</option>
                </select>
              </label>
            </li>
          </ul>
          <button v-if="toClasses.length" type="button" class="btn-primary" data-action="preview" :disabled="!chosen.length" @click="step = 'preview'">Pratinjau</button>
        </template>
        <template v-else>
          <p>{{ moving }} siswa akan ditempatkan di {{ label(toID) }}. Siswa yang sudah punya kelas di tahun itu dilewati. Penempatan {{ label(fromID) }} tetap sebagai riwayat.</p>
          <ul class="list" data-test="preview">
            <li v-for="c in chosen" :key="c.id"><div>{{ c.name }} ({{ c.students }} siswa) → <strong>{{ nameIn(toClasses, mapping[c.id]) }}</strong></div></li>
          </ul>
          <div class="row">
            <button type="button" class="btn-primary" data-action="promote" @click="promote">Naikkan {{ moving }} siswa</button>
            <button type="button" class="btn-secondary" @click="step = 'map'">Ubah pemetaan</button>
          </div>
        </template>
      </template>
    </fieldset>
  </section>
</template>

<style scoped src="../styles/classes.css"></style>
<style scoped>
.skipped { margin: 0; padding-left: var(--space-4); font-size: var(--text-sm); }
</style>
