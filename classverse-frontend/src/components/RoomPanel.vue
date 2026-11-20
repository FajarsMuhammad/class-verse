<script setup lang="ts">
import RemotePagination from './RemotePagination.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { errorMessage } from '@/api/client'
import { roomApi } from '@/api/rooms'
import type { Room } from '@/api/types'
import { fetchMap, type Building } from '@/realtime/mapdata'
import RoomForm from './RoomForm.vue'

const props = defineProps<{ orgId: string }>()
const rooms = ref<Room[]>([])
const pageNumber = ref(1)
const cursors = ref<(string | undefined)[]>([undefined])
async function movePage(delta: number) {
 if (delta > 0) cursors.value = [...cursors.value.slice(0, pageNumber.value), cursor.value ?? undefined]
 pageNumber.value += delta
 await loadPage()
}
async function loadPage() { await load(cursors.value[pageNumber.value - 1]) }
const buildings = ref<Building[]>([])
const cursor = ref<string | null>(null)
const loading = ref(false)
const error = ref('')
const status = ref('')
const formOpen = ref(false)
const editing = ref<Room>()
const archiving = ref<string | null>(null)
const busy = ref(false)
const buildingFilter = ref('')
const search = ref('')
let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(buildingFilter, () => void load())
watch(search, () => { clearTimeout(searchTimer); searchTimer = setTimeout(() => void load(), 250) })
const buildingName = (key: string) => buildings.value.find((b) => b.key === key)?.name ?? key
// One server page grouped by building, in campus map order.
const groups = computed(() => {
  const order = buildings.value.map((b) => b.key)
  const keys = [...new Set(rooms.value.map((r) => r.building_key))].sort((a, b) => (order.indexOf(a) + 1 || 999) - (order.indexOf(b) + 1 || 999))
  return keys.map((key) => ({ key, name: buildingName(key), rooms: rooms.value.filter((r) => r.building_key === key) }))
})

async function load(next?: string) {
  if (!next) { pageNumber.value = 1; cursors.value = [undefined] }
  loading.value = true
  error.value = ''
  try {
    const page = await roomApi.list(props.orgId, { cursor: next, building: buildingFilter.value, q: search.value })
    rooms.value = page.items
    cursor.value = page.next_cursor
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}
onMounted(async () => {
  await Promise.all([
    load(),
    fetchMap('campus')
      .then((map) => {
        buildings.value = map.buildings
      })
      .catch((e) => {
        error.value = errorMessage(e)
      }),
  ])
})
function edit(room?: Room) {
  editing.value = room
  formOpen.value = true
}
function saved(room: Room) {
  const index = rooms.value.findIndex((r) => r.id === room.id)
  if (index < 0) rooms.value.unshift(room)
  else rooms.value[index] = room
  status.value = 'Ruang disimpan.'
}
async function openNow(room: Room) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    saved(await roomApi.update(room.id, { scheduled_at: null }))
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
async function archive(room: Room) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await roomApi.archive(room.id)
    rooms.value = rooms.value.filter((r) => r.id !== room.id)
    archiving.value = null
    status.value = 'Ruang diarsipkan.'
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
function schedule(iso: string) {
  return new Intl.DateTimeFormat('id-ID', { dateStyle: 'medium', timeStyle: 'long' }).format(
    new Date(iso),
  )
}
</script>

<template>
  <section aria-labelledby="rooms-title">
    <header class="toolbar">
      <h2 id="rooms-title">Ruang sekolah</h2>
      <button class="btn-primary" :disabled="!buildings.length" @click="edit()">+ Ruang baru</button>
    </header>
    <div class="filters">
      <label>Gedung
        <select v-model="buildingFilter"><option value="">Semua gedung</option><option v-for="b in buildings" :key="b.key" :value="b.key">{{ b.name }}</option></select>
      </label>
      <label>Cari ruang <input v-model="search" type="search" maxlength="100" placeholder="Cari ruang…" /></label>
    </div>
    <p v-if="error" role="alert">
      {{ error }}
      <button class="btn-secondary" :disabled="loading" @click="load()">Coba lagi</button>
    </p>
    <p role="status">{{ status }}</p>
    <p v-if="loading" role="status">Memuat ruang…</p>
    <p v-else-if="!rooms.length && !error">
      {{ search || buildingFilter ? 'Ruang tidak ditemukan.' : 'Belum ada ruang. Pilih gedung dan peta untuk membuat ruang pertamamu.' }}
    </p>
    <section v-for="group in groups" :key="group.key" class="group" :aria-label="group.name">
      <h3 class="group-title">{{ group.name }}</h3>
      <div class="cards">
      <article v-for="room in group.rooms" :key="room.id" class="card">
        <div class="preview" :class="room.map_key" aria-hidden="true">
          <span v-for="n in room.map_key === 'garden' ? 3 : 8" :key="n"></span>
        </div>
        <div class="card-title">
          <h4>{{ room.name }}</h4>
          <span class="badge" :class="{ live: room.live, scheduled: !!room.scheduled_at && !room.live }">{{
            room.live ? `Live · ${room.people}` : room.scheduled_at ? 'Terjadwal' : 'Siap' }}</span>
        </div>
        <p>{{ buildingName(room.building_key) }} · kapasitas {{ room.capacity }}</p>
        <p v-if="room.scheduled_at">{{ schedule(room.scheduled_at) }}</p>
        <div class="actions">
          <RouterLink class="btn-primary map-link" :to="{ path: '/', query: { building: room.building_key } }">Masuk<span class="sr"> {{ room.name }}</span></RouterLink>
          <button class="btn-secondary" :disabled="busy" @click="edit(room)">Ubah<span class="sr"> {{ room.name }}</span></button>
          <button v-if="room.scheduled_at" class="btn-secondary" :disabled="busy" @click="openNow(room)">Buka sekarang</button>
          <button class="btn-secondary" :disabled="busy" @click="archiving = room.id">Arsipkan<span class="sr"> {{ room.name }}</span></button>
        </div>
        <div v-if="archiving === room.id" class="confirm">
          <p>Arsipkan {{ room.name }}? Ruang dengan peserta atau sesi terbuka tidak bisa diarsipkan.</p>
          <button class="btn-secondary" :disabled="busy" @click="archiving = null">Batal</button>
          <button class="btn-primary" :disabled="busy" @click="archive(room)">Ya, arsipkan</button>
        </div>
      </article>
      </div>
    </section>
    <button class="new-room" :disabled="!buildings.length" @click="edit()">
      <strong>Ruang baru</strong><span>Pilih gedung dan peta ruang, siap dalam satu menit.</span>
    </button>

    <RoomForm
      v-if="formOpen"
      :org-id="orgId"
      :buildings="buildings"
      :room="editing"
      @close="formOpen = false"
      @saved="saved"
    />
    <RemotePagination v-if="pageNumber > 1 || cursor" :page="pageNumber" :more="!!cursor" @move="movePage" />
  </section>
</template>

<style scoped>
.toolbar,
.card-title,
.actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  flex-wrap: wrap;
}
h2,
h3,
h4,
.new-room strong {
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  margin: 0;
}
h2 {
  font-size: var(--text-2xl);
}
h4,
.new-room strong {
  font-size: var(--text-lg);
}
p {
  color: var(--color-ink-muted);
}
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 290px), 1fr));
  gap: var(--space-5);
}
.card {
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
  padding: var(--space-4);
}
.preview {
  aspect-ratio: 1.8;
  border-radius: var(--radius-xl);
  background: var(--map-path);
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-3);
  padding: var(--space-4);
  margin-bottom: var(--space-4);
}
.preview span {
  background: var(--map-wood);
  border-radius: var(--radius-sm);
}
.preview.garden {
  background: var(--map-grass);
  grid-template-columns: repeat(3, 1fr);
  align-items: center;
}
.garden span {
  aspect-ratio: 1;
  border-radius: var(--radius-pill);
  background: var(--map-grass-dark);
}
.garden span:nth-child(2) {
  background: var(--map-water);
}
.garden span:nth-child(3) {
  background: var(--map-path);
  border-radius: var(--radius-sm);
}
.badge {
  border-radius: var(--radius-pill);
  padding: var(--space-1) var(--space-2);
  font-size: var(--text-sm);
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
}
.badge.live {
  background: var(--color-live-soft);
  color: var(--color-live-ink);
}
.badge.scheduled {
  background: var(--color-accent-soft);
  color: var(--color-accent-ink);
}
.map-link {
  display: flex;
  justify-content: center;
  align-items: center;
  text-decoration: none;
  box-sizing: border-box;
}
.actions {
  justify-content: start;
  margin-top: var(--space-3);
}
.new-room {
  min-height: 120px;
  width: 100%;
  border: 1px dashed var(--kv-primary);
  border-radius: var(--radius-2xl);
  background: transparent;
  color: var(--color-ink);
  padding: var(--space-6);
  font: inherit;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  gap: var(--space-4);
}
.new-room span {
  max-width: 28ch;
  color: var(--color-ink-muted);
}
.more {
  margin-top: var(--space-5);
}
.confirm {
  border-top: 1px solid var(--kv-line);
  margin-top: var(--space-3);
}
.filters { display: flex; flex-wrap: wrap; gap: var(--space-3); margin: var(--space-4) 0; }
.filters label { display: grid; gap: var(--space-1); font-size: var(--text-sm); }
.filters select, .filters input { min-height: var(--touch-min); padding: var(--space-2); border: 1px solid var(--kv-line); border-radius: var(--radius-md); background: var(--color-surface); color: var(--color-ink); font: inherit; }
.group { margin-bottom: var(--space-5); }
.group-title { margin: 0 0 var(--space-3); font-size: var(--text-md); }
.card { border: 1px solid var(--kv-line); }
.new-room { margin-top: var(--space-2); }
</style>
