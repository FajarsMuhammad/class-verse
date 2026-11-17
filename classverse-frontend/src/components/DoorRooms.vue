<script setup lang="ts">
import { ref, watch } from 'vue'
import { errorMessage } from '@/api/client'
import { roomApi } from '@/api/rooms'
import type { Room } from '@/api/types'
import type { Building } from '@/realtime/mapdata'

const props = defineProps<{
  orgId: string
  building: Building
  disabled: boolean
  atDoor: boolean
  // listMode is mode HP: entering needs no walk to the door.
  listMode?: boolean
}>()
const emit = defineEmits<{ travel: [roomId: string] }>()
const rooms = ref<Room[]>([])
const cursor = ref<string | null>(null)
const error = ref('')
const loading = ref(false)
let generation = 0
async function load(next?: string) {
  const id = ++generation
  loading.value = true
  error.value = ''
  try {
    const page = await roomApi.list(props.orgId, { building: props.building.key, cursor: next })
    if (id !== generation) return
    rooms.value = next ? [...rooms.value, ...page.items] : page.items
    cursor.value = page.next_cursor
  } catch (e) {
    if (id === generation) error.value = errorMessage(e)
  } finally {
    if (id === generation) loading.value = false
  }
}
watch(
  () => [props.orgId, props.building.key],
  () => {
    rooms.value = []
    cursor.value = null
    void load()
  },
  { immediate: true },
)
</script>

<template>
  <section aria-labelledby="door-title" class="door">
    <h2 id="door-title">{{ building.name }}</h2>
    <p role="status">
      {{
        listMode
          ? 'Pilih ruang untuk masuk.'
          : atDoor
          ? 'Kamu di dekat pintu. Pilih ruang untuk masuk.'
          : 'Jalan ke pintu gedung ini untuk masuk. Memilih gedung tidak memindahkan avatar.'
      }}
    </p>
    <p v-if="loading" role="status">Memuat ruang…</p>
    <p v-if="error" role="alert">
      {{ error }} <button class="btn-secondary" @click="load()">Coba lagi</button>
    </p>
    <p v-else-if="!loading && !rooms.length">Belum ada ruang di gedung ini.</p>
    <ul>
      <li v-for="room in rooms" :key="room.id">
        <strong>{{ room.name }}</strong>
        <span>{{ room.people }}/{{ room.capacity }} orang{{ room.live ? ' · Live' : '' }}</span>
        <span v-if="room.scheduled_at">{{
          new Date(room.scheduled_at).toLocaleString('id-ID', {
            dateStyle: 'medium',
            timeStyle: 'long',
          })
        }}</span>
        <button
          class="btn-primary"
          :disabled="disabled || room.people >= room.capacity"
          @click="emit('travel', room.id)"
        >
          Masuk ruang
        </button>
      </li>
    </ul>
    <button v-if="cursor" class="btn-secondary" :disabled="loading" @click="load(cursor)">
      Muat berikutnya
    </button>
  </section>
</template>

<style scoped>
.door { font-size: var(--text-sm); }
h2 {
  margin: 0;
  font: var(--weight-semibold) var(--text-base)/var(--line-tight) var(--font-display);
}
p,
span {
  margin: var(--space-1) 0;
  color: var(--color-ink-muted);
  font-size: var(--text-xs);
}
ul {
  margin: var(--space-3) 0 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: var(--space-2);
}
li {
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: center;
  column-gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border-soft);
  border-radius: var(--radius-lg);
}
li > strong, li > span { grid-column: 1; }
li > span { margin: 0; }
li > button { grid-column: 2; grid-row: 1 / span 3; min-height: 0; height: var(--control-sm); padding: 0 var(--space-3); border-radius: 10px; font-size: var(--text-xs); font-weight: var(--weight-semibold); }
</style>
