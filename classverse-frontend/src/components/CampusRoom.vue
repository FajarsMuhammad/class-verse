<script setup lang="ts">
import {
  computed,
  defineAsyncComponent,
  nextTick,
  onMounted,
  onBeforeUnmount,
  ref,
  watch,
} from 'vue'
import { useRoute } from 'vue-router'
import DoorRooms from '@/components/DoorRooms.vue'
import MapZoom from '@/components/MapZoom.vue'
import MapMinimap from '@/components/MapMinimap.vue'
import PlaceCard from '@/components/PlaceCard.vue'
import LearningPanel from '@/components/LearningPanel.vue'
import EmoteBar from '@/components/EmoteBar.vue'
import { useOrgStore } from '@/stores/org'
import { useAuthStore } from '@/stores/auth'
import type { Building, Seat } from '@/realtime/mapdata'

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
const auth = useAuthStore()
const learningClosed = ref(false)
const chatClosed = ref(false)
const route = useRoute()
const nearDoor = ref<Building | null>(null)
const zoom = ref(0.5)
const position = ref<{ x: number; y: number } | null>(null)
const otherPositions = ref<{ x: number; y: number }[]>([])
const campus = computed(() => !current.value || current.value.map_key === 'campus')
// ChatState is not reactive; chatVersion changes with it (room_state carries muted).
const muted = computed(() => {
  void chatVersion.value
  return chat.muted && !campus.value
})
const selectedKey = ref('')
const dismissedPlace = ref('')
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
  dismissedPlace.value = ''
  directoryOpen.value = false
  roomsOpen.value = false
}
function showPlaceRooms() {
  roomsOpen.value = true
}
function closePlace() {
  dismissedPlace.value = panelBuilding.value?.key ?? ''
  selectedKey.value = ''
}
function walkToSeat(seat: Seat) {
  if (campus.value || status.value !== 'open' || leaving.value || traveling.value) return
  if (!map.value?.seats.some((s) => s.key === seat.key && s.x === seat.x && s.y === seat.y)) return
  if ([...world.avatars.values()].some((a) => !a.hidden && Math.hypot(a.x-seat.x, a.y-seat.y) <= 18)) return
  walkTarget = { x: seat.x, y: seat.y, id: (walkTarget?.id ?? 0) + 1 }
}
function walkToPlace() {
  const building = panelBuilding.value
  if (!building || (building.staffOnly && !org.isTeacher) || status.value !== 'open') return
  walkTarget = { x: building.doorX, y: building.doorY, id: (walkTarget?.id ?? 0) + 1 }
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
const panelBuilding = computed(() => selectedBuilding.value ?? nearDoor.value)
const canEnter = computed(() => {
  const b = panelBuilding.value, p = position.value
  return !!b && !!p && Math.hypot(p.x-b.doorX, p.y-b.doorY) <= (b.interactRadius ?? 48)
})
let timer: ReturnType<typeof setInterval> | undefined

// Floating toolbars: the scene reads their height to keep the floor visible.
// Plain (non-reactive) values; Phaser polls them every frame.
const mapTop = ref<HTMLElement | null>(null)
const mapBottom = ref<HTMLElement | null>(null)
let inset = { top: 0, bottom: 0 }
const measure = new ResizeObserver(() => {
  const gap = 12 // --space-3 between a toolbar and the frame edge
  inset = {
    top: mapTop.value ? mapTop.value.offsetHeight + gap * 2 : 0,
    bottom: mapBottom.value ? mapBottom.value.offsetHeight + gap * 2 : 0,
  }
})
watch([mapTop, mapBottom], (els) => {
  measure.disconnect()
  for (const el of els) if (el) measure.observe(el)
})

// "Keluar ruang": walk to the exit, then travel. The server still requires the
// avatar at the exit (doorRadius); no teleport.
const EXIT_RADIUS = 40 // inside the server's 48 px doorRadius
const LEAVE_TIMEOUT_MS = 15000 // a 30-tile room crosses in under 6 s
let walkTarget: { x: number; y: number; id: number } | null = null
let leaveStarted = 0
const leaving = ref(false)
const leaveError = ref('')
function atExit() {
  const exit = map.value?.exit
  const self = world.self
  return !!exit && !!self && Math.hypot(self.x - exit.x, self.y - exit.y) <= EXIT_RADIUS
}
function leaveRoom() {
  const exit = map.value?.exit
  leaveError.value = ''
  if (!exit || atExit()) {
    void travel(props.roomId)
    return
  }
  leaving.value = true
  leaveStarted = Date.now()
  walkTarget = { x: exit.x, y: exit.y, id: (walkTarget?.id ?? 0) + 1 }
}
watch(campus, () => {
  leaving.value = false
  leaveError.value = ''
  walkTarget = null
})

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
    if (leaving.value && atExit()) {
      leaving.value = false
      void travel(props.roomId)
    } else if (leaving.value && Date.now() - leaveStarted > LEAVE_TIMEOUT_MS) {
      // Path blocked or the user walked elsewhere: stop, never travel later by surprise.
      leaving.value = false
      leaveError.value = 'Belum sampai Pintu keluar. Jalan ke sana, lalu tekan Keluar ruang lagi.'
    }
    const self = world.self
    position.value = self ? { x: self.x, y: self.y } : null
    if (!campus.value) otherPositions.value = [...world.avatars.values()]
      .filter((a) => !a.hidden).map((a) => ({ x: a.x, y: a.y }))
    const dismissed = map.value?.buildings.find((b) => b.key === dismissedPlace.value)
    if (dismissed && self && Math.hypot(self.x-dismissed.doorX, self.y-dismissed.doorY) > (dismissed.interactRadius ?? 48)) dismissedPlace.value = ''
    nearDoor.value =
      campus.value && self
        ? (map.value?.buildings.find((b) => Math.hypot(self.x - b.doorX, self.y - b.doorY) <= (b.interactRadius ?? 48)) ??
          null)
        : null
  }, 150)
})
onBeforeUnmount(() => {
  clearInterval(timer)
  measure.disconnect()
})
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
    <div class="layout" :class="[`show-${mobilePanel}`, { 'learning-closed': learningClosed, 'chat-closed': chatClosed }]">
      <LearningPanel v-model:closed="learningClosed" class="learning-panel" :name="auth.me?.display_name ?? ''"
        :avatar="world.selfAvatar" :teacher="org.isTeacher" :campus="campus" :room-name="current?.room_name"
        :zone-name="chat.zone.name" :seats="map?.seats ?? []" :self="position" :others="otherPositions"
        :seats-disabled="status !== 'open' || traveling || leaving || kicked" @walk="walkToSeat" />
      <div class="map-column">
        <div class="map-frame">
          <header ref="mapTop" class="map-top">
            <button
              v-if="!campus && !kicked"
              type="button"
              class="back"
              :aria-busy="leaving"
              :disabled="traveling || status !== 'open'"
              @click="leaveRoom"
            >
              <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
                <path
                  d="M15 5l-7 7 7 7"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2.5"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              {{ traveling ? 'Keluar…' : leaving ? 'Menuju pintu…' : 'Keluar ruang' }}
            </button>
            <div class="title">
              <h1 id="map-title">{{ campus ? schoolName : current?.room_name }}</h1>
              <p role="status" data-testid="connection-status">{{ statusText }}</p>
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
            <TeacherPanel v-else-if="org.isTeacher && !kicked" :room="room" floating />
          </header>
          <div v-if="map" class="stage">
            <GameCanvas
              :key="current?.room_id"
              :deps="{
                map,
                world,
                sendMove,
                currentZone: () => chat.zone.key,
                speaking: () => voice?.speaking.value ?? new Set(),
                inset: () => inset,
                walkTarget: () => walkTarget,
                zoom: () => zoom,
                selectBuilding,
              }"
              :label="`${campus ? 'Peta sekolah' : current?.room_name}. Fokus peta, gunakan WASD atau tombol panah untuk berjalan. Klik tujuan untuk mencari jalan.`"
            />
          </div>
          <p v-else class="loading">{{ error || 'Memuat peta sekolah…' }}</p>
          <MapZoom v-if="campus && map" v-model="zoom" class="map-zoom" />
          <MapMinimap v-if="campus && map" :map="map" :position="position" class="map-minimap" />
          <PlaceCard v-if="campus && panelBuilding && panelBuilding.key !== dismissedPlace && !directoryOpen && !roomsOpen"
            class="place-detail" :building="panelBuilding" :teacher="org.isTeacher" :at-door="canEnter"
            :disabled="traveling || status !== 'open'"
            :occupancy="buildings.find((item) => item.building_key === panelBuilding?.key)"
            @walk="walkToPlace" @rooms="showPlaceRooms" @close="closePlace" />
          <div v-if="kicked" class="connection-overlay" role="alert">
            <p>
              Kamu dikeluarkan dari sesi ini oleh pengajar. Minta izin pengajar untuk masuk lagi.
            </p>
            <button class="btn-primary" @click="rejoin">Buka peta sekolah</button>
          </div>
          <div v-else-if="map && status !== 'open'" class="connection-overlay" role="status">
            {{ statusText }}
          </div>
          <footer ref="mapBottom" class="map-bottom">
            <EmoteBar :send="room.sendEmote"
              :disabled="status !== 'open' || traveling || kicked || muted" />
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
            <p v-if="campus && selectedBuilding" class="hint">Tujuan: {{ selectedBuilding.name }}.</p>
            <p v-if="leaveError || error" class="float-note" role="alert">
              {{ leaveError || error }}
            </p>
          </footer>
        </div>
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
        <header class="panel-heading"><h2 v-if="!chatClosed">Chat</h2>
          <button class="btn-secondary" :aria-label="chatClosed ? 'Buka panel chat' : 'Ciutkan panel chat'"
            :aria-expanded="!chatClosed" @click="chatClosed = !chatClosed">{{ chatClosed ? '‹' : '›' }}</button>
        </header>
        <p v-if="!chatClosed" class="panel-note">Ngobrol dengan orang di sekitarmu</p>
        <ChatPanel
          v-show="!chatClosed"
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
/* Two rails float over the map. Each is one line with a fixed height, so a
   status change can never resize anything; the scene fits the floor between
   them (`inset`). Overflowing content scrolls sideways instead of wrapping. */
.map-top,
.map-bottom {
  position: absolute;
  z-index: 2;
  left: var(--space-3);
  right: var(--space-3);
  height: calc(var(--touch-min) + var(--space-4));
  box-sizing: border-box;
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: var(--space-3);
  padding: 0 var(--space-2);
  background: var(--color-surface);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-raise);
}
.map-top {
  top: var(--space-3);
}
.map-zoom {
  position: absolute;
  z-index: 2;
  top: calc(var(--touch-min) + var(--space-4) + var(--space-6));
  right: var(--space-3);
}
.map-bottom {
  bottom: var(--space-3);
}
.map-minimap {
  position: absolute;
  z-index: 2;
  bottom: calc(var(--touch-min) + var(--space-4) + var(--space-6));
  right: var(--space-3);
}
.float-note {
  position: absolute;
  right: 0;
  bottom: calc(100% + var(--space-2));
  max-width: min(28rem, 100%);
  margin: 0;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  background: var(--color-accent-soft);
  color: var(--color-accent-ink);
  font-size: var(--text-sm);
  font-weight: var(--weight-bold);
  box-shadow: var(--shadow-raise);
}
.title {
  flex: 1 1 auto;
  min-width: 6rem;
}
.title p {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-ink-muted);
}
.back {
  flex: none;
  min-height: var(--touch-min);
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  padding: 0 var(--space-3) 0 var(--space-2);
  border: 2px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-ink);
  font: inherit;
  font-weight: var(--weight-bold);
  cursor: pointer;
  white-space: nowrap;
}
.back:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.back:focus-visible {
  outline: 3px solid var(--color-primary-strong);
  outline-offset: 2px;
}
.place-detail {
  position: absolute;
  z-index: 3;
  bottom: calc(var(--space-14) + var(--space-10));
  left: var(--space-3);
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
h1 {
  margin: 0;
  font: var(--weight-semibold) var(--text-xl) / var(--line-tight) var(--font-display);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hint {
  flex: 1 1 0;
  min-width: 0;
  color: var(--color-ink-muted);
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
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
  grid-template-columns: 264px minmax(0, 1fr) 300px;
  gap: var(--space-4);
}
.layout.learning-closed { grid-template-columns: 76px minmax(0, 1fr) 300px; }
.layout.chat-closed { grid-template-columns: 264px minmax(0, 1fr) 76px; }
.layout.learning-closed.chat-closed { grid-template-columns: 76px minmax(0, 1fr) 76px; }
.panel-heading { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); }
.panel-heading h2 { margin: 0; font-size: var(--text-lg); }
.panel-heading button { flex: none; padding: 0; min-width: var(--touch-min); }
.panel-note { color: var(--kv-text-muted); font-size: var(--text-sm); }
/* HUD floats over the camera-follow viewport. */
.stage {
  position: absolute;
  inset: 0;
}
.map-frame {
  flex: 1;
  min-height: 0;
  width: 100%;
  position: relative;
  background: var(--kv-map-ground);
  border-radius: var(--kv-radius-map);
  overflow: clip;
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
  font-size: var(--text-sm);
  line-height: var(--line-normal);
}
.buildings {
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
  padding: var(--space-5);
  min-height: 0;
  box-sizing: border-box;
  position: absolute;
  z-index: 4;
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
  .learning-panel { display: none; }
  .layout, .layout.learning-closed, .layout.chat-closed, .layout.learning-closed.chat-closed {
    grid-template-columns: minmax(0, 1fr);
    min-height: 400px;
  }
  .show-map .chat-sidebar,
  .show-chat .map-column {
    display: none;
  }
}
</style>
