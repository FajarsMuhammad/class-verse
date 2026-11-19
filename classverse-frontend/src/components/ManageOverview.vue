<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { moduleApi } from '@/api/modules'
import { quizApi } from '@/api/quizzes'
import { classApi } from '@/api/classes'
import { assignmentApi } from '@/api/assignments'
import { errorMessage } from '@/api/client'
const props = defineProps<{ orgId: string }>()
const emit = defineEmits<{ select: [tool: string] }>()
const cards = ref<{ label: string; value: number; tool: string }[]>([])
const attention = ref<{ label: string; tool: string }[]>([])
const busy = ref(false)
const error = ref('')
let generation = 0
onBeforeUnmount(() => { generation++ })
async function load() {
  const g = ++generation
  busy.value = true; error.value = ''
  try {
    const [modules, quizzes, years, assignments] = await Promise.all([moduleApi.list(props.orgId, { drafts: true }), quizApi.list(props.orgId, { drafts: true }), classApi.years(props.orgId), assignmentApi.list(props.orgId)])
    const active = years.items.find(y => y.active)
    const [classes, students] = active ? await Promise.all([classApi.classes(props.orgId, active.id), classApi.roster(props.orgId, active.id, { limit: 1 })]) : [{ items: [] }, { total: 0 }]
    if (g !== generation) return
    cards.value = [
      { label: 'Modul terbit', value: modules.items.filter(m => m.published).length, tool: 'modules' },
      { label: 'Kuis terbit', value: quizzes.items.filter(q => q.published).length, tool: 'quizzes' },
      { label: 'Siswa', value: students.total, tool: 'students' },
      { label: 'Tugas aktif', value: assignments.items.filter(a => !a.cancelled).length, tool: 'assignments' },
    ]
    attention.value = [
      ...quizzes.items.filter(q => !q.question_count).map(q => ({ label: `${q.title} · belum punya soal`, tool: 'quizzes' })),
      ...classes.items.filter(c => !c.homeroom).map(c => ({ label: `${c.name} · belum ada wali kelas`, tool: 'classes' })),
      ...classes.items.filter(c => !c.students).map(c => ({ label: `${c.name} · belum ada siswa`, tool: 'classes' })),
      ...modules.items.filter(m => !m.published).map(m => ({ label: `${m.title} · modul draf`, tool: 'modules' })),
    ]
  } catch (e) { if (g === generation) error.value = errorMessage(e) }
  finally { if (g === generation) busy.value = false }
}
onMounted(load)
</script>
<template>
  <section class="overview" :aria-busy="busy" aria-labelledby="overview-title">
    <header><h2 id="overview-title">Beranda</h2><p>Ringkasan konten belajar, kelas, dan tugas sekolah.</p></header>
    <p v-if="busy" role="status">Memuat ringkasan…</p>
    <p v-if="error" role="alert">{{ error }} <button type="button" class="btn-secondary" @click="load">Coba lagi</button></p>
    <template v-if="!busy && !error">
      <div class="summary"><button v-for="card in cards" :key="card.tool" type="button" @click="emit('select', card.tool)"><span>{{ card.label }}</span><strong>{{ card.value }}</strong></button></div>
      <p class="muted">Ringkasan mengikuti batas daftar API. Jumlah siswa memakai tahun ajaran aktif; tugas aktif berarti belum dibatalkan.</p>
      <section class="attention"><h3>Perlu perhatian</h3><ul><li v-for="(item, index) in attention" :key="index"><span>{{ item.label }}</span><button type="button" class="btn-secondary" @click="emit('select', item.tool)">Buka</button></li></ul><p v-if="!attention.length">Tidak ada masalah pada data yang dimuat.</p></section>
      <section class="quick"><h3>Buat cepat</h3><div><button v-for="item in [{ label: 'Modul belajar', tool: 'modules' }, { label: 'Kuis', tool: 'quizzes' }, { label: 'Tugaskan modul', tool: 'assignments' }, { label: 'Undang anggota', tool: 'rooms' }]" :key="item.tool" type="button" class="btn-secondary" @click="emit('select', item.tool)">{{ item.label }}</button></div></section>
    </template>
  </section>
</template>
<style scoped>
h2 { font: var(--weight-semibold) var(--text-3xl)/var(--line-tight) var(--font-body); margin: 0; }
header p, .muted { color: var(--kv-text-muted); }
.muted { font-size: var(--text-sm); }
.summary { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: var(--space-4); }
.summary button, .attention, .quick { border: 1px solid var(--kv-line); border-radius: var(--radius-2xl); padding: var(--space-5); background: var(--kv-surface); color: var(--kv-ink); }
.summary button { text-align: left; font: inherit; cursor: pointer; }
.summary strong { display: block; font-size: var(--text-3xl); margin-top: var(--space-3); }
.attention, .quick { margin-top: var(--space-5); }
h3 { margin: 0 0 var(--space-3); }
ul { list-style: none; padding: 0; margin: 0; }
li { display: flex; justify-content: space-between; gap: var(--space-3); align-items: center; padding-block: var(--space-3); border-top: 1px solid var(--kv-line); }
.quick > div { display: flex; flex-wrap: wrap; gap: var(--space-3); }
button:focus-visible { outline: 2px solid var(--kv-primary); outline-offset: 2px; }
@media (max-width: 700px) { .summary { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
