<script setup lang="ts">
import { computed, watch } from 'vue'
import type { RoomHandle } from '@/realtime/useRoom'
import RoomTools from './RoomTools.vue'
import TeacherPanel from './TeacherPanel.vue'

const props = defineProps<{ orgId: string; userId: string; isAdmin: boolean; room: RoomHandle }>()
const emit = defineEmits<{ pinned: [] }>()
const active = defineModel<string>({ default: 'modules' })
const items = [
  { key: 'modules', label: 'Modul belajar', note: 'Susun materi, atur urutan belajar, dan sematkan modul ke ruang.', icon: 'M4 4h7v16H4zM13 4h7v16h-7' },
  { key: 'quizzes', label: 'Kuis', note: 'Buat soal, tentukan jawaban, dan terbitkan kuis.', icon: 'M9 4h6M9 3v4h6V3M7 5H5v16h14V5h-2M8 12h8M8 16h5' },
  { key: 'classes', label: 'Kelas', note: 'Buka satu kelas untuk mengatur siswa dan pengaturannya.', icon: 'M3 21V9l9-6 9 6v12M8 21v-6h8v6M8 11h.01M16 11h.01' },
  { key: 'students', label: 'Siswa', note: 'Cari siswa, lihat yang belum punya kelas, dan pindahkan kelas.', icon: 'M8 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8M2 21v-3a6 6 0 0 1 12 0v3M17 4a4 4 0 0 1 0 8M17 15a5 5 0 0 1 5 5v1' },
  { key: 'assignments', label: 'Tugas', note: 'Bagikan rangkaian belajar dan pantau progres tugas.', icon: 'M5 3h14v18H5zM8 8h8M8 12h8M8 16h5' },
  { key: 'rooms', label: 'Ruang & undangan', note: 'Atur ruang belajar dan undang anggota sekolah.', icon: 'M4 21V8l8-5 8 5v13M9 21v-7h6v7' },
  { key: 'books', label: 'Buku perpustakaan', note: 'Kelola koleksi dan peminjaman buku.', icon: 'M3 5l9-2 9 2v16l-9-2-9 2zM12 3v16' },
  { key: 'files', label: 'Berkas', note: 'Unggah dan kelola berkas pendukung materi.', icon: 'M6 3h8l4 4v14H6zM14 3v4h4' },
  { key: 'years', label: 'Tahun ajaran', note: 'Tambah tahun ajaran dan pilih tahun aktif sekolah.', icon: 'M4 5h16v16H4zM4 9h16M8 3v4M16 3v4' },
  { key: 'session', label: 'Kontrol sesi', note: 'Atur akses dan moderasi peserta di ruang ini.', icon: 'M4 7h16M4 17h16M8 4v6M16 14v6' },
] as const
const inRoom = computed(() => !!props.room.current.value && props.room.current.value.map_key !== 'campus' && !props.room.kicked.value)
const visible = computed(() => items.filter((item) => (item.key !== 'session' || inRoom.value) && (item.key !== 'years' || props.isAdmin)))
const current = computed(() => visible.value.find((item) => item.key === active.value) ?? items[0])
const tool = computed(() => active.value === 'modules' && props.room.current.value?.map_key === 'library' ? 'library-modules' : active.value)
watch(visible, (list) => {
  if (!list.some((item) => item.key === active.value)) active.value = 'modules'
})
function selectTool(key: string) {
 if (!window.dispatchEvent(new Event('kv-before-panel-change', { cancelable: true }))) return
 active.value=key
}
</script>

<template>
  <div class="manage-workspace">
    <nav class="manage-nav" aria-label="Submenu Kelola">
      <p class="nav-label">Alat pengajar</p>
      <button v-for="item in visible" :key="item.key" type="button" :data-tool="item.key"
        :aria-current="current.key === item.key ? 'page' : undefined" @click="selectTool(item.key)">
        <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path :d="item.icon" /></svg>
        <span>{{ item.label }}</span>
      </button>
    </nav>
    <section class="manage-content" aria-labelledby="manage-tool-title">
      <header class="tool-head" :class="{ 'sr-only': current.key !== 'session' }">
        <h3 id="manage-tool-title">{{ current.label }}</h3>
        <p>{{ current.note }}</p>
      </header>
      <TeacherPanel v-if="current.key === 'session'" :room="room" />
      <RoomTools v-else :key="tool" :tool="tool" :org-id="orgId" :user-id="userId" :is-admin="isAdmin" :room="room" @pinned="emit('pinned')" />
    </section>
  </div>
</template>

<style scoped>
/* Kelola owns navigation; individual managers own lists/forms. No custom dropdown keyboard model. */
.manage-workspace { display: grid; grid-template-columns: 200px minmax(0, 1fr); align-content: start; min-height: 0; flex: 1; }
.manage-nav { display: flex; flex-direction: column; gap: var(--space-1); padding: var(--space-3) var(--space-3) var(--space-3) 0; border-right: 1px solid var(--kv-line); align-self: start; }
.nav-label { margin: 0 0 var(--space-2) var(--space-3); font-size: var(--text-xs); color: var(--kv-text-muted); }
.manage-nav button { display: flex; align-items: center; gap: var(--space-2); min-height: var(--touch-min); padding: var(--space-2) var(--space-3); border: 0; border-radius: var(--control-radius); background: transparent; color: var(--kv-ink); font: var(--weight-semibold) var(--text-sm)/var(--line-normal) var(--font-body); cursor: pointer; text-align: left; }
.manage-nav button:hover { background: var(--color-surface-alt); }
.manage-nav button[aria-current='page'] { background: var(--color-primary-soft); color: var(--color-primary-strong); }
.manage-nav button:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: -3px; }
svg { flex: none; fill: none; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
.manage-content { min-width: 0; padding: var(--space-3) 0 var(--space-3) var(--space-6); }
.tool-head { padding-bottom: var(--space-4); border-bottom: 1px solid var(--kv-line); }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
h3 { margin: 0; font: var(--weight-semibold) var(--text-lg)/var(--line-normal) var(--font-display); }
.tool-head p { margin: var(--space-1) 0 0; font-size: var(--text-sm); color: var(--kv-text-muted); }
@media (max-width: 1000px) {
  .manage-workspace { grid-template-columns: 1fr; }
  .manage-nav { flex-direction: row; flex-wrap: wrap; padding: 0 0 var(--space-3); border-right: 0; border-bottom: 1px solid var(--kv-line); }
  .nav-label { flex-basis: 100%; margin-left: 0; }
  .manage-content { padding: var(--space-4) 0 0; }
}
</style>
