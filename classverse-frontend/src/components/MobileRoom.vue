<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import ChatPanel from '@/components/ChatPanel.vue'
import DoorRooms from '@/components/DoorRooms.vue'
import ReportDialog from '@/components/ReportDialog.vue'
import TeacherPanel from '@/components/TeacherPanel.vue'
import { avatarURL } from '@/game/avatars'
import type { ChatLine } from '@/realtime/chat'
import type { Building } from '@/realtime/mapdata'
import { count, nearby, places } from '@/realtime/places'
import type { RoomHandle } from '@/realtime/useRoom'
import { useOrgStore } from '@/stores/org'

// Mode HP (docs/design.md, slice 8): building and zone lists plus chat, no map.
const props = defineProps<{ room: RoomHandle; roomId: string; schoolName: string; statusText: string }>()
const {
  status,
  map,
  world,
  online,
  error,
  current,
  buildings,
  travel,
  traveling,
  chat,
  chatVersion,
  chatError,
  sendChat,
  seat,
  kicked,
  rejoin,
} = props.room
const org = useOrgStore()

// Positions arrive in snapshots without a reactive signal; refresh lists twice a second.
const tick = ref(0)
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  timer = setInterval(() => tick.value++, 500)
})
onBeforeUnmount(() => clearInterval(timer))

const campus = computed(() => !current.value || current.value.map_key === 'campus')
// Building names are only on the campus map; remember them for the room header.
const buildingList = ref<Building[]>([])
watch(map, (m) => {
  if (m?.buildings.length) buildingList.value = m.buildings
})
const openKey = ref('')
// Collapse on every room change so an opened building reloads fresh counts.
watch(current, () => (openKey.value = ''))
const zoneKey = computed(() => {
  void chatVersion.value
  return chat.zone.key
})

const near = computed(() => {
  void tick.value
  void online.value
  return nearby(world, map.value?.nearRadius ?? 160)
})
const lastNear = computed(() => {
  void chatVersion.value
  const lines = chat.lines.near
  return lines[lines.length - 1]
})
const placeList = computed(() => {
  void tick.value
  void online.value
  return map.value && !campus.value ? places(map.value, world) : []
})
const myPlace = computed(() => placeList.value.find((p) => p.key === zoneKey.value))
const buildingName = computed(
  () => buildingList.value.find((b) => b.key === current.value?.building_key)?.name ?? '',
)
function countOf(key: string) {
  return buildings.value.find((b) => b.building_key === key)
}

const reporting = ref<ChatLine | null>(null)
const reportNotice = ref('')
function reported() {
  reporting.value = null
  reportNotice.value = 'Laporan terkirim ke pengajar dan admin.'
}
</script>

<template>
  <section class="mobile" aria-labelledby="mobile-title">
    <header class="heading">
      <button
        v-if="!campus && !kicked"
        type="button"
        class="back"
        aria-label="Kembali ke peta sekolah"
        :disabled="traveling || status !== 'open'"
        @click="travel(roomId)"
      >
        <svg viewBox="0 0 24 24" width="24" height="24" aria-hidden="true">
          <path
            d="M15 5l-7 7 7 7"
            fill="none"
            stroke="currentColor"
            stroke-width="2.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
      <div>
        <h1 id="mobile-title">{{ campus ? schoolName : current?.room_name }}</h1>
        <p role="status" data-testid="connection-status">
          <template v-if="error">{{ error }}</template>
          <template v-else-if="status !== 'open'">{{ statusText }}</template>
          <template v-else-if="campus">Kamu di alun-alun · {{ online }} online</template>
          <template v-else
            >{{ buildingName ? `${buildingName} · ` : '' }}{{ online }} orang</template
          >
        </p>
      </div>
    </header>

    <div v-if="kicked" class="card" role="alert">
      <p>Kamu dikeluarkan dari sesi ini oleh pengajar. Minta izin pengajar untuk masuk lagi.</p>
      <button class="btn-primary" @click="rejoin">Buka peta sekolah</button>
    </div>

    <template v-else-if="campus">
      <section class="card" aria-labelledby="near-title">
        <div class="card-head">
          <h2 id="near-title">Di dekatmu</h2>
          <span>{{ near.length }} orang</span>
        </div>
        <p v-if="!near.length" class="muted">Belum ada orang di dekatmu.</p>
        <ul v-else class="faces" aria-label="Orang di dekatmu">
          <li v-for="a in near.slice(0, 6)" :key="a.id">
            <img :src="avatarURL(a.avatar)" :alt="a.name" width="40" height="40" style="object-fit: contain; image-rendering: pixelated" />
          </li>
        </ul>
        <p v-if="lastNear" class="preview">{{ lastNear.fromName }}: {{ lastNear.text }}</p>
      </section>

      <h2 class="label">Pilih gedung</h2>
      <ul class="buildings">
        <li
          v-for="b in buildingList"
          :key="b.key"
          :class="[b.roof || 'outdoor', { open: openKey === b.key }]"
        >
          <button
            type="button"
            class="building"
            :aria-expanded="openKey === b.key"
            :aria-controls="`rooms-${b.key}`"
            @click="openKey = openKey === b.key ? '' : b.key"
          >
            <span class="bar" aria-hidden="true"></span>
            <span class="name">{{ b.name }}</span>
            <span v-if="b.staffOnly" class="meta">Khusus pengajar</span>
            <span v-else-if="countOf(b.key)?.live_rooms" class="live"
              >{{ countOf(b.key)?.live_rooms }} live</span
            >
            <span v-else class="meta">{{ countOf(b.key)?.people ?? 0 }} orang</span>
          </button>
          <div v-if="openKey === b.key && org.active" :id="`rooms-${b.key}`" class="rooms">
            <DoorRooms
              :org-id="org.active.id"
              :building="b"
              :disabled="traveling || status !== 'open'"
              at-door
              list-mode
              @travel="travel"
            />
          </div>
        </li>
      </ul>
    </template>

    <template v-else>
      <details v-if="org.isTeacher" class="card">
        <summary>Kontrol pengajar</summary>
        <TeacherPanel :room="room" />
      </details>
      <h2 class="label">Pilih tempat</h2>
      <div class="places" role="group" aria-label="Pilih tempat">
        <button
          v-for="p in placeList"
          :key="p.key"
          type="button"
          class="place"
          :aria-pressed="p.key === zoneKey"
          :disabled="status !== 'open' || traveling"
          @click="p.key !== zoneKey && seat(p.key)"
        >
          {{ p.name }} · {{ count(p, zoneKey) }}
        </button>
      </div>
      <section v-if="myPlace" class="card mates" aria-live="polite">
        <ul v-if="myPlace.people.length" class="faces" aria-hidden="true">
          <li v-for="a in myPlace.people.slice(0, 4)" :key="a.id">
            <img :src="avatarURL(a.avatar)" alt="" width="40" height="40" style="object-fit: contain; image-rendering: pixelated" />
          </li>
        </ul>
        <div>
          <strong>Kamu di {{ myPlace.name }}</strong>
          <p class="muted">
            {{
              myPlace.people.length
                ? myPlace.people.map((a) => a.name).join(', ')
                : 'Belum ada orang lain di sini.'
            }}
          </p>
        </div>
      </section>
    </template>

    <section v-if="!kicked" class="card chat" aria-label="Chat">
      <ChatPanel
        :chat="chat"
        :version="chatVersion"
        :error="chatError"
        :disabled="status !== 'open'"
        :campus="campus"
        :teacher="org.isTeacher"
        @send="sendChat"
        @report="reporting = $event"
      />
      <p v-if="reportNotice" role="status" class="muted">{{ reportNotice }}</p>
    </section>
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
  </section>
</template>

<style scoped>
.mobile {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-4);
}
.heading {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}
h1 {
  margin: 0;
  font: var(--weight-semibold) var(--text-xl) / var(--line-tight) var(--font-display);
}
.heading p,
.muted,
.meta {
  margin: 0;
  color: var(--color-ink-muted);
}
.back {
  min-width: var(--touch-min);
  min-height: var(--touch-min);
  display: grid;
  place-items: center;
  border: 0;
  border-radius: var(--radius-lg);
  background: var(--color-surface);
  color: var(--color-ink);
  cursor: pointer;
}
.card {
  background: var(--color-surface);
  border-radius: var(--radius-xl);
  padding: var(--space-4);
}
.card summary {
  min-height: var(--touch-min);
  display: flex;
  align-items: center;
  font-weight: var(--weight-bold);
  cursor: pointer;
}
.card-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}
h2 {
  margin: 0;
  font-size: var(--text-lg);
}
.label {
  font-size: var(--text-md);
  color: var(--color-ink-muted);
}
.faces {
  list-style: none;
  display: flex;
  gap: var(--space-1);
  padding: 0;
  margin: var(--space-3) 0;
}
.preview {
  margin: 0;
  padding: var(--space-3);
  border-radius: var(--radius-lg);
  background: var(--color-surface-alt);
  font-size: var(--text-md);
  overflow-wrap: anywhere;
}
.buildings {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: var(--space-3);
}
.buildings > li {
  --bar: var(--color-speaking);
  background: var(--color-surface);
  border: 2px solid transparent;
  border-radius: var(--radius-xl);
}
.buildings > li.open {
  border-color: var(--color-primary);
}
.buildings > li.classroom {
  --bar: var(--roof-classroom);
}
.buildings > li.staff {
  --bar: var(--roof-staff);
}
.buildings > li.library {
  --bar: var(--roof-library);
}
.buildings > li.hall {
  --bar: var(--roof-hall);
}
.buildings > li.canteen {
  --bar: var(--roof-canteen);
}
.buildings > li.lab { --bar: var(--roof-lab); }
.buildings > li.studio { --bar: var(--roof-studio); }
.buildings > li.mosque { --bar: var(--roof-mosque); }
.buildings > li.clinic { --bar: var(--roof-clinic); }
.building {
  width: 100%;
  min-height: var(--touch-min);
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-4);
  border: 0;
  background: transparent;
  color: var(--color-ink);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.bar {
  width: var(--space-2);
  align-self: stretch;
  min-height: var(--space-8);
  border-radius: var(--radius-pill);
  background: var(--bar);
}
.name {
  flex: 1;
  font-weight: var(--weight-bold);
  font-size: var(--text-lg);
}
.live {
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-pill);
  background: var(--color-live-soft);
  color: var(--color-live-ink);
  font-weight: var(--weight-bold);
}
.rooms {
  padding: 0 var(--space-4) var(--space-4);
}
.places {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.place {
  min-height: var(--touch-min);
  padding: var(--space-2) var(--space-4);
  border: 2px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
  color: var(--color-ink);
  font: inherit;
  font-weight: var(--weight-bold);
  cursor: pointer;
}
.place[aria-pressed='true'] {
  border-color: var(--color-accent);
  background: var(--color-accent-soft);
  color: var(--color-accent-ink);
}
.mates {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}
.mates .faces {
  margin: 0;
}
.chat {
  min-height: 420px;
  display: flex;
  flex-direction: column;
}
</style>
