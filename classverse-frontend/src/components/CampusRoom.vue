<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import DoorRooms from '@/components/DoorRooms.vue'
import { useOrgStore } from '@/stores/org'
import type { Building } from '@/realtime/mapdata'

import GameCanvas from '@/components/GameCanvas.vue'
import { useRoom } from '@/realtime/useRoom'

const props = defineProps<{ roomId: string; schoolName: string }>()
const {
  status,
  map,
  world,
  online,
  error,
  sendMove,
  start,
  current,
  buildings,
  travel,
  traveling,
} = useRoom()
const org = useOrgStore()
const route = useRoute()
const nearDoor = ref<Building | null>(null)
const campus = computed(() => !current.value || current.value.map_key === 'campus')
const selectedKey = ref('')
watch(
  () => route.query.building,
  (key) => {
    selectedKey.value = typeof key === 'string' ? key : ''
  },
  { immediate: true },
)
const selectedBuilding = computed(() =>
  map.value?.buildings.find((b) => b.key === selectedKey.value),
)
const panelBuilding = computed(() => selectedBuilding.value ?? nearDoor.value)
const canEnter = computed(
  () => !!panelBuilding.value && panelBuilding.value.key === nearDoor.value?.key,
)
let timer: ReturnType<typeof setInterval> | undefined
const statusText = computed(
  () =>
    ({
      connecting: 'Menghubungkan…',
      open: `${online.value} orang online`,
      reconnecting: 'Koneksi terputus. Menghubungkan kembali…',
      replaced: 'Kelasverse dibuka di tab lain. Muat ulang untuk memakai tab ini.',
      denied: 'Tidak dapat masuk peta sekolah. Pilih sekolah kembali.',
      closed: 'Koneksi ditutup.',
    })[status.value],
)

onMounted(() => {
  void start(props.roomId, 'campus')
  timer = setInterval(() => {
    const self = world.self
    nearDoor.value =
      campus.value && self
        ? (map.value?.buildings.find((b) => Math.hypot(self.x - b.doorX, self.y - b.doorY) <= 48) ??
          null)
        : null
  }, 150)
})
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <section aria-labelledby="map-title">
    <header class="heading">
      <div>
        <h1 id="map-title">{{ campus ? schoolName : current?.room_name }}</h1>
        <p role="status" data-testid="connection-status">{{ error || statusText }}</p>
      </div>
      <span v-if="campus" class="scope">Peta sekolah</span>
      <button
        v-else
        class="btn-secondary"
        :disabled="traveling || status !== 'open'"
        @click="travel(roomId)"
      >
        Kembali ke peta sekolah
      </button>
    </header>
    <div class="layout">
      <div>
        <div class="map-frame">
          <GameCanvas
            v-if="map"
            :key="current?.room_id"
            :deps="{ map, world, sendMove }"
            :label="`${campus ? 'Peta sekolah' : current?.room_name}. Fokus peta, gunakan WASD atau tombol panah untuk berjalan. Klik tujuan untuk mencari jalan.`"
          />
          <p v-else class="loading">{{ error || 'Memuat peta sekolah…' }}</p>
          <div v-if="map && status !== 'open'" class="connection-overlay" role="status">
            {{ statusText }}
          </div>
        </div>
        <p class="hint">
          Gerak dengan WASD, tombol panah, atau klik tujuan.
          {{
            campus
              ? 'Jalan ke pintu gedung untuk memilih ruang.'
              : 'Jalan ke pintu keluar sebelum kembali ke peta sekolah.'
          }}
          <span v-if="campus && selectedBuilding"
            >Tujuan: {{ selectedBuilding.name }}. Jalan ke pintunya; tidak ada teleport.</span
          >
        </p>
      </div>
      <aside class="buildings" aria-labelledby="buildings-title">
        <h2 id="buildings-title">{{ campus ? 'Gedung' : 'Ruang' }}</h2>
        <DoorRooms
          v-if="campus && panelBuilding && org.active"
          :org-id="org.active.id"
          :building="panelBuilding"
          :disabled="!canEnter || traveling || status !== 'open'"
          :at-door="canEnter"
          @travel="travel"
        />
        <p v-else-if="campus">
          Pilih gedung untuk melihat ruang, lalu jalan ke pintunya untuk masuk.
        </p>
        <ul v-if="campus">
          <li v-for="b in map?.buildings ?? []" :key="b.key" :class="b.key">
            <button
              class="building-choice"
              :aria-pressed="panelBuilding?.key === b.key"
              @click="selectedKey = b.key"
            >
              {{ b.name }}
            </button>
            <span>{{ b.staffOnly ? 'Khusus pengajar dan admin' : b.description }}</span>
            <span v-if="buildings.find((item) => item.building_key === b.key)"
              >{{ buildings.find((item) => item.building_key === b.key)?.people }} orang ·
              {{ buildings.find((item) => item.building_key === b.key)?.live_rooms }} ruang
              live</span
            >
          </li>
        </ul>
        <p v-if="!campus">
          {{ online }} orang di ruang. Chat dan zona diskusi hadir pada slice berikutnya.
        </p>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.building-choice {
  width: 100%;
  min-height: var(--touch-min);
  border: 2px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-ink);
  font: inherit;
  font-weight: var(--weight-bold);
  text-align: left;
  cursor: pointer;
  padding: var(--space-2);
}
.building-choice[aria-pressed='true'] {
  border-color: var(--color-primary);
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
}
.heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  margin-bottom: var(--space-5);
}
h1 {
  margin: 0;
  font: var(--weight-semibold) var(--text-3xl)/var(--line-tight) var(--font-display);
}
.heading p,
.hint {
  color: var(--color-ink-muted);
  margin: var(--space-2) 0 0;
}
.scope {
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-pill);
  font-weight: var(--weight-bold);
}
.layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 280px;
  gap: var(--space-5);
  align-items: start;
  container-type: inline-size;
}
.map-frame {
  aspect-ratio: 8 / 5;
  width: 100%;
  position: relative;
  background: var(--map-grass);
  border-radius: var(--radius-2xl);
  overflow: hidden;
}
.loading {
  padding: var(--space-8);
}
.connection-overlay {
  position: absolute;
  inset: 0;
  display: grid;
  place-content: center;
  padding: var(--space-8);
  background: var(--color-surface-alt);
  opacity: 0.95;
  text-align: center;
  font-weight: var(--weight-bold);
}
.hint {
  font-size: var(--text-md);
  text-align: center;
  line-height: var(--line-relaxed);
}
.buildings {
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
  padding: var(--space-5);
  min-height: 0;
  box-sizing: border-box;
  max-height: calc((100cqw - 280px - var(--space-5)) * 0.625);
  overflow-y: auto;
}
h2 {
  font: var(--weight-semibold) var(--text-2xl)/var(--line-tight) var(--font-display);
  margin: 0 0 var(--space-4);
}
ul {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: var(--space-3);
}
li {
  padding: var(--space-3);
  border-left: var(--space-1) solid var(--roof-classroom);
  border-radius: var(--radius-sm);
  background: var(--color-bg);
}
li span {
  display: block;
  font-size: var(--text-sm);
  color: var(--color-ink-muted);
  margin-top: var(--space-1);
}
.ruang-guru {
  border-color: var(--roof-staff);
}
.perpustakaan {
  border-color: var(--roof-library);
}
.aula {
  border-color: var(--roof-hall);
}
.kantin {
  border-color: var(--roof-canteen);
}
.taman,
.lapangan {
  border-color: var(--color-speaking);
}
@media (max-width: 900px) {
  .layout {
    grid-template-columns: minmax(0, 1fr);
  }
  .scope {
    display: none;
  }
  .buildings {
    max-height: none;
  }
}
</style>
