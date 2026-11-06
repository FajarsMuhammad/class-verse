<script setup lang="ts">
import { computed, defineAsyncComponent, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import DoorRooms from '@/components/DoorRooms.vue'
import { useOrgStore } from '@/stores/org'
import type { Building } from '@/realtime/mapdata'

import ChatPanel from '@/components/ChatPanel.vue'
import ReportDialog from '@/components/ReportDialog.vue'
import type { ChatLine } from '@/realtime/chat'
import MobileRoom from '@/components/MobileRoom.vue'
import TeacherPanel from '@/components/TeacherPanel.vue'
import VoiceBar from '@/components/VoiceBar.vue'
import { useRoom } from '@/realtime/useRoom'
import { useVoice } from '@/realtime/useVoice'

// Phaser is loaded only when the map is shown (never in mode HP).
const GameCanvas = defineAsyncComponent(() => import('@/components/GameCanvas.vue'))
// Mode HP (design slice 8, M8-01): phones get lists instead of the map. Decided
// once per room start; the connection mode cannot change afterwards.
const listMode =
  typeof window.matchMedia === 'function' &&
  window.matchMedia('(pointer: coarse) and (max-width: 767px)').matches

const props = defineProps<{ roomId: string; schoolName: string }>()
const room = useRoom()
// Voice is web only for now (mode HP voice is deferred with the phone polish).
const voice = listMode ? null : useVoice(room)
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
  chat,
  chatVersion,
  chatError,
  sendChat,
  kicked,
  rejoin,
} = room
const org = useOrgStore()
const route = useRoute()
const nearDoor = ref<Building | null>(null)
const campus = computed(() => !current.value || current.value.map_key === 'campus')
const selectedKey = ref('')
const directoryButton = ref<HTMLButtonElement | null>(null)
const closeButton = ref<HTMLButtonElement | null>(null)
const directoryOpen = ref(false)
const roomsOpen = ref(false)
const mobilePanel = ref<'map' | 'chat'>('map')
function toggleDirectory() {
  directoryOpen.value = !directoryOpen.value
  roomsOpen.value = false
}
async function closeDirectory() {
  directoryOpen.value = false
  roomsOpen.value = false
  await nextTick()
  directoryButton.value?.focus()
}
watch(
  () => directoryOpen.value || roomsOpen.value,
  async (open) => {
    if (open) {
      await nextTick()
      closeButton.value?.focus()
    }
  },
)
function selectBuilding(key: string) {
  selectedKey.value = key
  directoryOpen.value = false
  roomsOpen.value = true
}
watch(campus, () => {
  directoryOpen.value = false
  roomsOpen.value = false
  selectedKey.value = ''
})
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
const panelBuilding = computed(() => nearDoor.value ?? selectedBuilding.value)
const canEnter = computed(
  () => !!panelBuilding.value && panelBuilding.value.key === nearDoor.value?.key,
)
let timer: ReturnType<typeof setInterval> | undefined

const reporting = ref<ChatLine | null>(null)
const reportNotice = ref('')
function reported() {
  reporting.value = null
  reportNotice.value = 'Laporan terkirim ke pengajar dan admin.'
}
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
  void start(props.roomId, 'campus', listMode ? 'list' : 'map')
  if (listMode) return
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
  <MobileRoom
    v-if="listMode"
    :room="room"
    :room-id="roomId"
    :school-name="schoolName"
    :status-text="statusText"
  />
  <section v-else class="workspace" aria-labelledby="map-title">
    <header class="heading">
      <div>
        <h1 id="map-title">{{ campus ? schoolName : current?.room_name }}</h1>
        <p role="status" data-testid="connection-status">{{ error || statusText }}</p>
      </div>
      <button
        v-if="campus"
        ref="directoryButton"
        class="btn-secondary"
        :aria-expanded="directoryOpen"
        aria-controls="building-directory"
        @click="toggleDirectory"
      >
        Daftar gedung
      </button>
      <button
        v-else-if="!kicked"
        class="btn-secondary"
        :disabled="traveling || status !== 'open'"
        @click="travel(roomId)"
      >
        Kembali ke peta sekolah
      </button>
    </header>
    <div class="mobile-switch" aria-label="Tampilan ruang">
      <button
        class="btn-secondary"
        :aria-pressed="mobilePanel === 'map'"
        @click="mobilePanel = 'map'"
      >
        Peta
      </button>
      <button
        class="btn-secondary"
        :aria-pressed="mobilePanel === 'chat'"
        @click="mobilePanel = 'chat'"
      >
        Chat
      </button>
    </div>
    <div class="layout" :class="`show-${mobilePanel}`">
      <div class="map-column">
        <div class="map-frame">
          <GameCanvas
            v-if="map"
            :key="current?.room_id"
            :deps="{
              map,
              world,
              sendMove,
              currentZone: () => chat.zone.key,
              speaking: () => voice?.speaking.value ?? new Set(),
            }"
            :label="`${campus ? 'Peta sekolah' : current?.room_name}. Fokus peta, gunakan WASD atau tombol panah untuk berjalan. Klik tujuan untuk mencari jalan.`"
          />
          <p v-else class="loading">{{ error || 'Memuat peta sekolah…' }}</p>
          <div v-if="campus && nearDoor && !directoryOpen && !roomsOpen" class="door-prompt">
            <strong>{{ nearDoor.name }}</strong>
            <button class="btn-primary" @click="roomsOpen = true">Pilih ruang</button>
          </div>
          <div v-if="kicked" class="connection-overlay" role="alert">
            <p>
              Kamu dikeluarkan dari sesi ini oleh pengajar. Minta izin pengajar untuk masuk lagi.
            </p>
            <button class="btn-primary" @click="rejoin">Buka peta sekolah</button>
          </div>
          <div v-else-if="map && status !== 'open'" class="connection-overlay" role="status">
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
      <aside
        v-if="campus && (directoryOpen || roomsOpen)"
        id="building-directory"
        class="buildings"
        aria-label="Pilihan gedung dan ruang"
        @keydown.esc="closeDirectory"
      >
        <button ref="closeButton" class="btn-secondary" @click="closeDirectory">
          Tutup pilihan
        </button>
        <h2 v-if="directoryOpen">Daftar gedung</h2>
        <DoorRooms
          v-if="roomsOpen && panelBuilding && org.active"
          :org-id="org.active.id"
          :building="panelBuilding"
          :disabled="!canEnter || traveling || status !== 'open'"
          :at-door="canEnter"
          @travel="travel"
        />
        <p v-if="directoryOpen">
          Pilih gedung untuk melihat ruang, lalu jalan ke pintunya untuk masuk.
        </p>
        <ul v-if="directoryOpen">
          <li v-for="b in map?.buildings ?? []" :key="b.key" :class="b.key">
            <button
              class="building-choice"
              :aria-pressed="panelBuilding?.key === b.key"
              @click="selectBuilding(b.key)"
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
      </aside>
      <aside class="chat-sidebar" aria-label="Chat ruang">
        <h2>Chat</h2>
        <VoiceBar
          v-if="voice && !campus && !kicked"
          :voice="voice"
          :chat="chat"
          :version="chatVersion"
          :self-id="world.selfId"
          :teacher="org.isTeacher"
          :disabled="status !== 'open'"
          @broadcast="room.voiceBroadcast"
        />
        <TeacherPanel v-if="!campus && org.isTeacher" :room="room" />
        <ChatPanel
          class="side-chat"
          :chat="chat"
          :version="chatVersion"
          :error="chatError"
          :disabled="status !== 'open'"
          :campus="campus"
          :teacher="org.isTeacher"
          @send="sendChat"
          @report="reporting = $event"
        />
        <p v-if="reportNotice" role="status" class="room-meta">{{ reportNotice }}</p>
        <ReportDialog
          v-if="reporting && current"
          :target-id="reporting.fromId"
          :target-name="reporting.fromName"
          :room-id="current.room_id"
          :message-id="reporting.messageId"
          :message-text="reporting.text"
          @close="reporting = null"
          @reported="reported"
        />
      </aside>
    </div>
  </section>
</template>

<style scoped>
.room-meta {
  margin: 0 0 var(--space-3);
  color: var(--color-ink-muted);
}
.workspace {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.mobile-switch {
  display: none;
}
.map-column {
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.chat-sidebar {
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: var(--space-4);
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
}
.side-chat {
  flex: 1;
  min-height: 0;
}
.door-prompt {
  position: absolute;
  bottom: var(--space-4);
  left: var(--space-4);
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3);
  background: var(--color-surface);
  border-radius: var(--radius-lg);
}
.campus-chat {
  margin-top: var(--space-5);
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
  padding: var(--space-5);
  height: 340px;
  box-sizing: border-box;
}
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
  flex: 1;
  min-height: 0;
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: var(--space-4);
}
.map-frame {
  flex: 1;
  min-height: 0;
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
  position: absolute;
  z-index: 2;
  top: var(--space-3);
  left: var(--space-3);
  width: min(360px, calc(100% - var(--space-6)));
  max-height: calc(100% - var(--space-6));
  border: 2px solid var(--color-border);
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
  .mobile-switch {
    display: flex;
    gap: var(--space-2);
    margin-bottom: var(--space-3);
  }
  .layout {
    grid-template-columns: minmax(0, 1fr);
    min-height: 400px;
  }
  .show-map .chat-sidebar,
  .show-chat .map-column {
    display: none;
  }
  .heading {
    flex-wrap: wrap;
  }
}
</style>
