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
import BuildingModules from '@/components/BuildingModules.vue'
import LearningPanel from '@/components/LearningPanel.vue'
import RoomLearningPin from '@/components/RoomLearningPin.vue'
import EmoteBar from '@/components/EmoteBar.vue'
import { useOrgStore } from '@/stores/org'
import { useAuthStore } from '@/stores/auth'
import type { Building, Seat } from '@/realtime/mapdata'

import ChatPanel from '@/components/ChatPanel.vue'
import LibraryCatalog from '@/components/LibraryCatalog.vue'
import RoomStations from '@/components/RoomStations.vue'
import RoomTools from '@/components/RoomTools.vue'
import SideMenu, { type SideItem } from '@/components/SideMenu.vue'
import type { Shelf } from '@/api/types'
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
const shelf = ref<Shelf | ''>('')
const route = useRoute()
const nearDoor = ref<Building | null>(null)
const zoom = ref(0.5)
const position = ref<{ x: number; y: number } | null>(null)
const otherPositions = ref<{ x: number; y: number }[]>([])
const campus = computed(() => !current.value || current.value.map_key === 'campus')
const library = computed(() => current.value?.map_key === 'library')
const stationRoom = computed(() => ['science', 'staff'].includes(current.value?.map_key ?? ''))
const contentRoom = computed(() => library.value || stationRoom.value)
const station = ref('')
// One side menu (docs/screens/sidemenu.png) replaces the old left/right columns so the map gets the width.
// Staff tools appear only where they belong: teaching tools in Ruang Guru, collection tools in Perpustakaan.
// Placement only; the server authorizes every call by role, never by the avatar's room.
const icons = {
  learn: 'M4 6l8-3 8 3-8 3zM6 8v6c0 2 3 4 6 4s6-2 6-4V8',
  chat: 'M4 5h16v11H9l-5 4z',
  catalog: 'M5 4h5v16H5zM12 4h3v16h-3zM17 5l3 1-3 14-3-1',
  station: 'M12 3l2 5h5l-4 4 2 6-5-3-5 3 2-6-4-4h5z',
  modules: 'M4 5h7v14H4zM13 5h7v14h-7',
  quizzes: 'M12 3l9 5-9 5-9-5zM6 11v5l6 3 6-3v-5',
  classes: 'M8 11a3 3 0 100-6 3 3 0 000 6zM16 11a3 3 0 100-6 3 3 0 000 6zM2 20c0-3 3-5 6-5s6 2 6 5M14 15c3 0 8 1 8 5',
  assignments: 'M9 4h6v3H9zM6 6h12v15H6zM9 12l2 2 4-4',
  gradebook: 'M5 20V10M12 20V4M19 20v-7',
  rooms: 'M4 20V8l8-5 8 5v12M9 20v-6h6v6',
  files: 'M6 3h8l4 4v14H6zM14 3v4h4',
  moderation: 'M12 3l8 3v6c0 5-4 8-8 9-4-1-8-4-8-9V6z',
} as const
const toolPlace = computed(() => !org.isTeacher || kicked.value ? null : current.value?.map_key === 'staff' ? 'staff' : library.value ? 'library' : null)
// Staff "Modul" holds the live-session pin in every interior; Ruang Guru/Perpustakaan add their managers there.
const pinPlace = computed(() => org.isTeacher && !kicked.value && !campus.value)
const toolItems = computed<SideItem[]>(() => [
  ...(pinPlace.value ? [{ key: 'modules', label: 'Modul', icon: icons.modules, wide: toolPlace.value !== null }] : []),
  ...(toolPlace.value === 'staff'
    ? [
        { key: 'quizzes', label: 'Kuis', icon: icons.quizzes, wide: true },
        { key: 'classes', label: 'Kelas', icon: icons.classes, wide: true },
        { key: 'assignments', label: 'Tugas', icon: icons.assignments, wide: true },
        { key: 'gradebook', label: 'Rapor', icon: icons.gradebook, wide: true },
        { key: 'rooms', label: 'Ruang', icon: icons.rooms, wide: true },
        { key: 'files', label: 'Berkas', icon: icons.files, wide: true },
      ]
    : toolPlace.value === 'library'
      ? [
          { key: 'books', label: 'Kelola buku', icon: icons.catalog, wide: true },
          { key: 'files', label: 'Berkas', icon: icons.files, wide: true },
        ]
      : []),
])
// Unread dot on Chat while its panel is closed (ChatState is not reactive; chatVersion tracks it).
const seenChat = ref(0)
const chatCount = computed(() => {
  void chatVersion.value
  return Object.values(chat.lines).reduce((n, l) => n + l.length, 0)
})
// Participants find the pin at the top of Belajar; a dot marks a pin they have not opened the panel for yet.
const pinKey = computed(() => room.pin?.value ? `${room.pin.value.module_id}:${room.pin.value.step_id}` : '')
const seenPin = ref('')
const sideItems = computed<SideItem[]>(() => [
  { key: 'learn', label: 'Belajar', icon: icons.learn, dot: !org.isTeacher && !!pinKey.value && pinKey.value !== seenPin.value && sideItem.value !== 'learn' },
  { key: 'chat', label: 'Chat', icon: icons.chat, keep: true, dot: sideItem.value !== 'chat' && chatCount.value > seenChat.value },
  ...(contentRoom.value ? [{ key: 'catalog', label: library.value ? 'Katalog' : 'Stasiun', icon: library.value ? icons.catalog : icons.station, wide: library.value }] : []),
  ...toolItems.value,
])
// The panel remembers being collapsed so the map stays wide; when open, content rooms start on their catalog.
const SIDE_KEY = 'kv-side-collapsed'
const sideItem = ref<string | null>(localStorage.getItem(SIDE_KEY) === '1' ? null : 'learn')
watch(sideItem, (key) => {
  localStorage.setItem(SIDE_KEY, key ? '0' : '1')
  if (key === 'chat') seenChat.value = chatCount.value
  if (key === 'learn') seenPin.value = pinKey.value
})
watch(pinKey, (k) => { if (sideItem.value === 'learn') seenPin.value = k })
watch(chatCount, (n) => { if (sideItem.value === 'chat') seenChat.value = n })
watch(() => current.value?.room_id, () => {
  if (!sideItem.value) return
  if (!sideItems.value.some((i) => i.key === sideItem.value)) sideItem.value = 'learn'
  if (contentRoom.value) sideItem.value = 'catalog'
})
function selectStation(id: string) {
  station.value = id
  sideItem.value = 'catalog'
}
function selectShelf(cat: string) {
  shelf.value = cat as Shelf
  sideItem.value = 'catalog'
}
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
const modulesOf = ref<Building | null>(null)
const progressKey = ref(0)
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
watch(() => current.value?.room_id, () => {
  shelf.value = ''
  station.value = ''
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
    <div class="layout" :class="{ 'menu-open': !!sideItem }">
      <SideMenu v-model:active="sideItem" class="side" :items="sideItems" :label="org.isTeacher ? 'Menu ruang dan alat guru' : 'Menu ruang'"
        :link="toolPlace === 'staff' ? { to: '/moderasi', label: 'Moderasi', icon: icons.moderation } : undefined">
        <template #learn>
          <RoomLearningPin v-if="!org.isTeacher && !campus && !kicked && org.active && room.pin?.value" :key="current?.room_id"
            :room="room" :org-id="org.active.id" :staff="false" />
          <LearningPanel embedded :org-id="org.active?.id" :progress-key="progressKey" :name="auth.me?.display_name ?? ''"
            :avatar="world.selfAvatar" :teacher="org.isTeacher" :campus="campus" :room-name="current?.room_name"
            :zone-name="chat.zone.name" :seats="map?.seats ?? []" :self="position" :others="otherPositions"
            :seats-disabled="status !== 'open' || traveling || leaving || kicked" @walk="walkToSeat" />
        </template>
        <template #chat>
          <p class="panel-note">Ngobrol dengan orang di sekitarmu</p>
          <ChatPanel class="side-chat" :chat="chat" :version="chatVersion" :error="chatError" :disabled="status !== 'open'"
            :campus="campus" :teacher="org.isTeacher" @send="sendChat" @report="reporting = $event" />
          <p v-if="reportNotice" role="status" class="room-meta">{{ reportNotice }}</p>
        </template>
        <template #catalog>
          <LibraryCatalog v-if="library && org.active" v-model:shelf="shelf" class="side-catalog" :org-id="org.active.id" />
          <RoomStations v-else-if="stationRoom && org.active" :key="current?.room_id" class="side-catalog" :org-id="org.active.id"
            :map-key="current!.map_key" :station="station" :teacher="org.isTeacher" @select="selectStation"
            @manage="sideItem = 'modules'" />
        </template>
        <template #modules>
          <RoomLearningPin v-if="org.active" :key="current?.room_id" :room="room" :org-id="org.active.id" staff />
          <RoomTools v-if="toolPlace && org.active && auth.me" :tool="toolPlace === 'library' ? 'library-modules' : 'modules'"
            :org-id="org.active.id" :user-id="auth.me.id" :is-admin="org.active.role === 'admin'" />
        </template>
        <template v-for="t in toolItems.filter((i) => i.key !== 'modules')" :key="t.key" #[t.key]>
          <RoomTools v-if="org.active && auth.me" :tool="t.key" :org-id="org.active.id" :user-id="auth.me.id"
            :is-admin="org.active.role === 'admin'" />
        </template>
      </SideMenu>
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
        <aside
          v-if="campus && (directoryOpen || roomsOpen)"
          id="building-directory"
          class="buildings"
          aria-label="Pilihan gedung dan ruang"
          @keydown.esc="closeDirectory"
        >
          <header class="dropdown-head">
            <h2 v-if="directoryOpen">Daftar gedung</h2>
            <span v-else></span>
            <button ref="closeButton" class="btn-secondary close" aria-label="Tutup pilihan" @click="closeDirectory">×</button>
          </header>
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
              selectShelf,
              selectStation,
              station: () => station,
              shelf: () => shelf,
            }"
            :label="`${campus ? 'Peta sekolah' : current?.room_name}. Fokus peta, gunakan WASD atau tombol panah untuk berjalan. Klik dua kali tujuan untuk mencari jalan.`"
          />
        </div>
        <p v-else class="loading">{{ error || 'Memuat peta sekolah…' }}</p>
        <MapZoom v-if="campus && map" v-model="zoom" class="map-zoom" />
        <MapMinimap v-if="campus && map" :map="map" :position="position" class="map-minimap" />
        <PlaceCard v-if="campus && panelBuilding && panelBuilding.key !== dismissedPlace && !directoryOpen && !roomsOpen"
          class="place-detail" :building="panelBuilding" :teacher="org.isTeacher" :at-door="canEnter"
          :disabled="traveling || status !== 'open'"
          :occupancy="buildings.find((item) => item.building_key === panelBuilding?.key)"
          @walk="walkToPlace" @rooms="showPlaceRooms" @close="closePlace" @modules="modulesOf = panelBuilding ?? null" />
        <BuildingModules v-if="modulesOf && org.active" :org-id="org.active.id" :building-key="modulesOf.key"
          :building-name="modulesOf.name" @close="modulesOf = null; progressKey++" />
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
          <p v-if="map?.quiet && !campus" class="quiet-chip">Zona tenang — suara mati, chat tetap bisa</p>
          <VoiceBar
            v-else-if="voice && !campus && !kicked"
            :voice="voice"
            :chat="chat"
            :version="chatVersion"
            :self-id="world.selfId"
            :teacher="org.isTeacher"
            :disabled="status !== 'open'"
            @broadcast="room.voiceBroadcast"
          />
          <p v-if="leaveError || error" class="float-note" role="alert">
            {{ leaveError || error }}
          </p>
        </footer>
        </div>
      </div>
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
.map-column {
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.side-chat {
  flex: 1;
  min-height: 0;
}
.side-catalog {
  flex: 1;
  overflow-y: auto;
  margin-top: var(--space-3);
}
/* Two rails float over the map. Each is one line with a fixed height, so a
   status change can never resize anything; the scene fits the floor between
   them (`inset`). Overflowing content scrolls sideways instead of wrapping. */
/* Rails are transparent; only their pill-shaped children take space and clicks,
   so the map stays visible (and clickable) between them. */
.map-top,
.map-bottom {
  position: absolute;
  z-index: 2;
  left: var(--space-3);
  right: var(--space-3);
  box-sizing: border-box;
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: var(--space-2);
  pointer-events: none;
}
.map-top > *,
.map-bottom > * {
  pointer-events: auto;
  box-shadow: var(--shadow-raise);
  border-radius: var(--radius-pill);
}
.map-bottom { justify-content: center; }
.quiet-chip {
  margin: 0;
  display: flex;
  align-items: center;
  min-height: var(--touch-min);
  padding: 0 var(--space-4);
  background: var(--color-surface);
  color: var(--color-ink);
  font-size: var(--text-sm);
  font-weight: var(--weight-semibold);
  white-space: nowrap;
}
.map-top {
  top: var(--space-3);
  overflow-x: auto;
  scrollbar-width: none;
}
.map-top .title { flex-shrink: 0; }
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
  /* Below the top rail, mirroring the zoom control on the right. */
  top: calc(var(--touch-min) + var(--space-4) + var(--space-6));
  left: var(--space-3);
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
  flex: 0 1 auto;
  min-width: 6rem;
  display: flex;
  align-items: baseline;
  gap: var(--space-2);
  min-height: var(--touch-min);
  padding: var(--space-1) var(--space-4);
  box-sizing: border-box;
  background: var(--color-surface);
  overflow: hidden;
}
.title + * { margin-left: auto; flex: none; white-space: nowrap; }
.title p {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-ink-muted);
  white-space: nowrap;
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
  display: flex;
  gap: var(--space-3);
}
.side { flex: none; }
.map-column { flex: 1; min-width: 0; }
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
.dropdown-head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); margin-bottom: var(--space-3); }
.dropdown-head h2 { margin: 0; font-size: var(--text-lg); }
.dropdown-head .close { flex: none; min-width: var(--touch-min); padding: 0; }
.buildings {
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
  padding: var(--space-5);
  min-height: 0;
  box-sizing: border-box;
  position: absolute;
  z-index: 4;
  /* Dropdown under the "Daftar gedung" button at the right end of the top rail. */
  top: calc(var(--touch-min) + var(--space-3) + var(--space-2));
  right: var(--space-3);
  width: min(360px, calc(100% - var(--space-6)));
  max-height: calc(100% - var(--touch-min) - var(--space-8));
  box-shadow: var(--shadow-raise);
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
  /* Teacher controls overflow narrow screens: scroll the rail, keep pills whole. */
  .map-top { overflow-x: auto; padding: var(--space-1) 0; scrollbar-width: none; }
  .map-top > * { flex: none; }
  .map-top .title { flex: 0 1 auto; max-width: 55vw; }
  .quiet-chip { white-space: normal; min-width: 0; padding: var(--space-1) var(--space-3); border-radius: var(--radius-lg); }
  /* Keep the place name readable; the status stays available to screen readers. */
  .title p {
    position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%);
  }
  /* Narrow screens: rail across the top, the open panel above the map. */
  .layout { flex-direction: column; min-height: 400px; }
  .layout.menu-open .side { max-height: 55%; }
}
</style>
