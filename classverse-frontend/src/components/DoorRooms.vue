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
.door {
  border-top: 2px solid var(--color-border-soft);
  margin-top: var(--space-5);
  padding-top: var(--space-5);
}
h2 {
  font: var(--weight-semibold) var(--text-xl)/var(--line-tight) var(--font-display);
}
p,
span {
  color: var(--color-ink-muted);
}
ul {
  padding: 0;
  list-style: none;
  display: grid;
  gap: var(--space-4);
}
li {
  display: grid;
  gap: var(--space-2);
}
</style>
