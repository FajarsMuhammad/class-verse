<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted } from 'vue'
import type { GuestClient, GuestSession } from '@/api/guests'
import { useRoom } from '@/realtime/useRoom'
import { useVoice } from '@/realtime/useVoice'
import ChatPanel from './ChatPanel.vue'
import VoiceBar from './VoiceBar.vue'

const GameCanvas = defineAsyncComponent(() => import('./GameCanvas.vue'))
const props = defineProps<{ session: GuestSession; client: GuestClient }>()
const emit = defineEmits<{ leave: [] }>()
const room = useRoom(props.client.getToken)
const voice = useVoice({ ...room, voiceToken: props.client.voiceToken })
const { status, map, world, current, error, kicked, people, online, chat, chatVersion, chatError } = room
const closed = computed(() => kicked.value || status.value === 'denied' || status.value === 'closed')
const names = computed(() => people.value.map(p => `${p.name}${p.role === 'guest' ? ' · Tamu' : ''}`))
const zone = computed(() => { void chatVersion.value; return chat.zone.key })
onMounted(() => room.start(props.session.room_id, props.session.map_key))
</script>

<template>
  <main class="guest-room">
    <header>
      <div><h1>{{ session.room_name }}</h1><p>{{ session.org_name }} · {{ session.display_name }} · Tamu</p></div>
      <button class="btn-secondary" @click="emit('leave')">Tinggalkan pertemuan</button>
    </header>
    <section v-if="closed" class="ended" role="alert">
      <h2>{{ kicked ? 'Kamu dikeluarkan dari sesi ini' : 'Akses pertemuan berakhir' }}</h2>
      <p>{{ error || 'Hubungi pengajar jika kamu memerlukan akses kembali.' }}</p>
    </section>
    <template v-else>
      <p v-if="error || status !== 'open'" role="status">{{ error || 'Menghubungkan ke pertemuan…' }}</p>
      <div class="meeting">
        <section class="map-area" aria-label="Peta ruang pertemuan">
          <GameCanvas v-if="map && current" :key="current.room_id" label="Peta pertemuan. Ketuk dua kali tujuan untuk berjalan, atau gunakan WASD."
            :deps="{ map, world, sendMove: room.sendMove, currentZone: () => zone, speaking: () => voice?.speaking.value ?? new Set() }" />
        </section>
        <ChatPanel :chat="chat" :version="chatVersion" :error="chatError" :disabled="status !== 'open'" :campus="false" guest :online="online" :people="names" @send="room.sendChat" />
      </div>
      <footer v-if="voice">
        <VoiceBar :voice="voice" :chat="chat" :version="chatVersion" :self-id="world.selfId" :teacher="false" :disabled="status !== 'open'" />
        <span>Ketuk dua kali tujuan untuk berjalan, atau gunakan WASD. Tamu hanya dapat mengikuti ruang ini.</span>
      </footer>
    </template>
  </main>
</template>

<style scoped>
.guest-room { height: 100dvh; display: flex; flex-direction: column; background: var(--color-bg); padding: var(--space-4); gap: var(--space-3); box-sizing: border-box; }
header { display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--kv-line); padding-bottom: var(--space-3); }
h1 { margin: 0; font: var(--weight-bold) var(--text-xl)/var(--line-tight) var(--font-display); }
p { margin: var(--space-2) 0; color: var(--kv-text-muted); }
.meeting { flex: 1; min-height: 0; display: grid; grid-template-columns: minmax(0, 1fr) minmax(18rem, 24rem); gap: var(--space-4); }
.map-area { min-height: 0; }
footer { display: flex; align-items: center; gap: var(--space-3); }
footer span { font-size: var(--text-sm); color: var(--kv-text-muted); }
/* Narrow screens: the map keeps most of the height and chat sits under it. */
@media (max-width: 767px) { header { align-items: start; flex-wrap: wrap; } .meeting { grid-template-columns: 1fr; grid-template-rows: minmax(45dvh, 1fr) minmax(0, 1fr); } footer { flex-wrap: wrap; } }
</style>
