<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { errorMessage } from '@/api/client'
import { roomApi } from '@/api/rooms'
import type { Room } from '@/api/types'
import { fetchMap, type Building } from '@/realtime/mapdata'
import RoomForm from './RoomForm.vue'

const props = defineProps<{ orgId: string }>()
const rooms = ref<Room[]>([])
const buildings = ref<Building[]>([])
const cursor = ref<string | null>(null)
const loading = ref(false)
const error = ref('')
const status = ref('')
const formOpen = ref(false)
const editing = ref<Room>()
const archiving = ref<string | null>(null)
const busy = ref(false)

async function load(next?: string) {
  loading.value = true
  error.value = ''
  try {
    const page = await roomApi.list(props.orgId, { cursor: next })
    rooms.value = next ? [...rooms.value, ...page.items] : page.items
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
      <button class="btn-primary" :disabled="!buildings.length" @click="edit()">
        + Buat ruang
      </button>
    </header>
    <p v-if="error" role="alert">
      {{ error }}
      <button class="btn-secondary" :disabled="loading" @click="load()">Coba lagi</button>
    </p>
    <p role="status">{{ status }}</p>
    <p v-if="loading" role="status">Memuat ruang…</p>
    <p v-else-if="!rooms.length && !error">
      Belum ada ruang. Pilih gedung dan peta untuk membuat ruang pertamamu.
    </p>
    <div class="cards">
      <article v-for="room in rooms" :key="room.id" class="card">
        <div class="preview" :class="room.map_key" aria-hidden="true">
          <span v-for="n in room.map_key === 'garden' ? 3 : 8" :key="n"></span>
        </div>
        <div class="card-title">
          <h3>{{ room.name }}</h3>
          <span
            class="badge"
            :class="{ live: room.live, scheduled: !!room.scheduled_at && !room.live }"
            >{{
              room.live ? `Live · ${room.people}` : room.scheduled_at ? 'Terjadwal' : 'Siap'
            }}</span
          >
        </div>
        <p>
          {{ buildings.find((b) => b.key === room.building_key)?.name ?? room.building_key }} ·
          kapasitas {{ room.capacity }}
        </p>
        <p v-if="room.scheduled_at">{{ schedule(room.scheduled_at) }}</p>
        <RouterLink
          class="btn-primary map-link"
          :to="{ path: '/', query: { building: room.building_key } }"
          >Lihat di peta</RouterLink
        >
        <div class="actions">
          <button class="btn-secondary" :disabled="busy" @click="edit(room)">Ubah</button>
          <button
            v-if="room.scheduled_at"
            class="btn-secondary"
            :disabled="busy"
            @click="openNow(room)"
          >
            Buka sekarang
          </button>
          <button class="btn-secondary" :disabled="busy" @click="archiving = room.id">
            Arsipkan
          </button>
        </div>
        <div v-if="archiving === room.id" class="confirm">
          <p>
            Arsipkan {{ room.name }}? Ruang dengan peserta atau sesi terbuka tidak bisa diarsipkan.
          </p>
          <button class="btn-secondary" :disabled="busy" @click="archiving = null">Batal</button>
          <button class="btn-primary" :disabled="busy" @click="archive(room)">Ya, arsipkan</button>
        </div>
      </article>
      <button class="new-room" :disabled="!buildings.length" @click="edit()">
        <strong>Ruang baru</strong><span>Pilih gedung dan peta ruang, siap dalam satu menit.</span
        ><span>Buat ruang</span>
      </button>
    </div>
    <button v-if="cursor" class="btn-secondary more" :disabled="loading" @click="load(cursor)">
      Muat ruang berikutnya
    </button>
    <RoomForm
      v-if="formOpen"
      :org-id="orgId"
      :buildings="buildings"
      :room="editing"
      @close="formOpen = false"
      @saved="saved"
    />
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
.new-room strong {
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  margin: 0;
}
h2 {
  font-size: var(--text-2xl);
}
h3,
.new-room strong {
  font-size: var(--text-xl);
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
  min-height: 300px;
  border: 1px dashed var(--color-border);
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
.new-room span:last-child {
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
  border-radius: var(--radius-lg);
  padding: var(--space-4);
  font-weight: var(--weight-bold);
}
.more {
  margin-top: var(--space-5);
}
.confirm {
  border-top: 1px solid var(--kv-line);
  margin-top: var(--space-3);
}
</style>
