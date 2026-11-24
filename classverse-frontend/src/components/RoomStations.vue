<script setup lang="ts">
import { hasMore } from '@/api/page'
import RemotePagination from './RemotePagination.vue'
import { computed, getCurrentInstance, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { roomLayout } from '@/game/roomLayout'
import { moduleApi } from '@/api/modules'
import { errorMessage } from '@/api/client'
import type { Module } from '@/api/types'
import ModulePath from './ModulePath.vue'

const props = defineProps<{ orgId: string; mapKey: string; station: string; teacher: boolean }>()
// manage opens the Modul tool beside the map; without a listener (phone list mode) it links to Kelola.
const emit = defineEmits<{ select: [id: string]; manage: [] }>()
const inPlace = !!getCurrentInstance()?.vnode.props?.onManage
const stations = computed(() => roomLayout(props.mapKey)?.stations ?? [])
const selected = computed(() => stations.value.find((s) => s.id === props.station) ?? stations.value[0])
const staff = computed(() => props.mapKey === 'staff')
const showModules = ref(false)
const modules = ref<Module[]>([])
const pageNumber = ref(1)
const more = ref(false)
function move(delta: number) { pageNumber.value += delta; void load() }
const loading = ref(false)
const error = ref('')
let generation = 0
async function load() {
  const mine = ++generation
  if (!staff.value || !props.teacher) return
  loading.value = true
  error.value = ''
  try {
    const page = await moduleApi.list(props.orgId, { drafts: true, limit: 20, offset: (pageNumber.value - 1) * 20 })
    if (mine === generation) { modules.value = page.items; more.value = hasMore((pageNumber.value - 1) * 20, page) }
  } catch (err) {
    if (mine === generation) error.value = errorMessage(err)
  } finally {
    if (mine === generation) loading.value = false
  }
}
watch([() => props.orgId, staff, () => props.teacher], load, { immediate: true })
watch(() => selected.value?.id, () => { showModules.value = false })
const description = computed(() => {
  if (staff.value) return ({
    kerja: 'Susun materi, video, dan berkas. Terbitkan modul ke gedung pelajaran.',
    rapat: 'Tempat berdiskusi dengan pengajar lain. Gunakan chat atau suara yang tersedia di ruang.',
    studio: 'Sudut persiapan materi video. Tambahkan tautan YouTube melalui pengelola modul.',
    nilai: 'Rapor dan pemeriksaan tugas belum tersedia.',
  } as Record<string, string>)[selected.value?.id ?? '']
  return 'Jelajahi materi IPA lewat modul sekolah. Stasiun ini merupakan area belajar; simulasi interaktif belum tersedia.'
})
</script>

<template>
  <section class="stations" :aria-label="staff ? 'Alat guru' : 'Stasiun Menara Sains'">
    <div class="station-list" role="group" aria-label="Pilih stasiun">
      <button v-for="s in stations" :key="s.id" type="button" class="station-tab" :aria-pressed="selected?.id === s.id"
        @click="emit('select', s.id)">{{ s.label }}</button>
    </div>
    <div class="intro">
      <p class="eyebrow">{{ staff ? 'Ruang Guru' : 'Jalur belajar IPA' }}</p>
      <h3>{{ selected?.label }}</h3>
      <p>{{ description }}</p>
      <button v-if="staff && teacher && inPlace" type="button" class="btn-primary" @click="emit('manage')">Buat dan kelola modul</button>
      <RouterLink v-else-if="staff && teacher" to="/kelola-ruang" class="btn-primary">Buat dan kelola modul</RouterLink>
      <button v-else-if="!staff" type="button" class="btn-primary" @click="showModules = !showModules"
        :aria-expanded="showModules">{{ showModules ? 'Tutup modul' : 'Lihat modul IPA' }}</button>
    </div>
    <section v-if="staff && teacher" class="school-modules" aria-label="Modul sekolah">
      <h3>Modul sekolah</h3>
      <p v-if="loading" role="status">Memuat modul…</p>
      <p v-else-if="error" class="alert" role="alert">{{ error }} <button type="button" class="link" @click="load">Coba lagi</button></p>
      <p v-else-if="!modules.length" class="note">Belum ada modul. Buat modul pertama melalui Kelola.</p>
      <component :is="inPlace ? 'button' : RouterLink" v-for="m in modules" :key="m.id" v-bind="inPlace ? { type: 'button', onClick: () => emit('manage') } : { to: '/kelola-ruang' }" class="module-row">
        <strong>{{ m.title }}</strong><span>{{ m.published ? 'Terbit' : 'Draf' }}</span>
        <small>{{ m.level }} · {{ m.steps.length }} langkah</small>
      </component>
      <RemotePagination :page="pageNumber" :more="more" :busy="loading" @move="move" />
    </section>
    <ModulePath v-if="!staff && showModules" :org-id="orgId" building-key="sains" building-name="Menara Sains" />
    <p class="note">{{ staff ? 'Modul, kuis, kelas, tugas, dan rapor ada di menu alat guru. Rekaman dan statistik kelas menyusul.' : 'Modul tersedia untuk semua stasiun. Pengelompokan modul per stasiun belum tersedia.' }}</p>
  </section>
</template>

<style scoped>
.stations { display: grid; align-content: start; gap: var(--space-4); }
.station-list { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.station-tab { min-height: var(--touch-min); padding: var(--space-2) var(--space-3); border: 1px solid var(--color-border); border-radius: var(--radius-pill); background: var(--color-surface); color: var(--color-ink); font: inherit; cursor: pointer; }
.station-tab[aria-pressed='true'] { background: var(--kv-subject-ipa); color: var(--kv-surface); border-color: var(--kv-ink); }
.intro { display: grid; gap: var(--space-3); }
h3 { margin: 0; font-family: var(--font-display); font-size: var(--text-xl); }
p { margin: 0; line-height: var(--line-normal); }
.eyebrow { color: var(--kv-subject-ipa); font-weight: var(--weight-semibold); }
.note { color: var(--color-ink-muted); font-size: var(--text-sm); }
.school-modules { display: grid; gap: var(--space-3); border-top: 1px solid var(--color-border-soft); padding-top: var(--space-4); }
.module-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: var(--space-2); padding: var(--space-3); border: 1px solid var(--color-border-soft); border-radius: var(--radius-lg); color: var(--color-ink); text-decoration: none; background: transparent; font: inherit; text-align: left; cursor: pointer; }
.module-row span { font-size: var(--text-xs); color: var(--kv-subject-ipa); }
.module-row small { grid-column: 1 / -1; color: var(--color-ink-muted); }
.btn-primary { text-align: center; text-decoration: none; background: var(--kv-subject-ipa); }
</style>
