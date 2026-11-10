<script setup lang="ts">
import { computed, ref } from 'vue'

import { errorMessage } from '@/api/client'
import { roomApi } from '@/api/rooms'
import type { RoomHandle } from '@/realtime/useRoom'

// Teacher controls of a classroom/garden, shared by the map and mode HP.
// floating: shown in the map toolbar, where the participant list opens as a popover.
const props = defineProps<{ room: RoomHandle; floating?: boolean }>()
const {
  status,
  traveling,
  current,
  chat,
  chatVersion,
  gather,
  setLocked,
  people,
  mutedIds,
  moderate,
  kick,
  banned,
  allow,
} = props.room

const allChat = computed(() => {
  void chatVersion.value
  return chat.allChatEnabled
})
const locked = computed(() => {
  void chatVersion.value
  return chat.locked
})
const toggling = ref(false)
const toggleError = ref('')
async function toggleAllChat() {
  if (!current.value || toggling.value) return
  toggling.value = true
  toggleError.value = ''
  try {
    await roomApi.update(current.value.room_id, { all_chat_enabled: !allChat.value })
  } catch (e) {
    toggleError.value = errorMessage(e)
  } finally {
    toggling.value = false
  }
}
const participants = computed(() => people.value.filter((p) => p.role === 'participant'))
const confirmKick = ref<{ id: string; name: string } | null>(null)
function doKick() {
  if (confirmKick.value) kick(confirmKick.value.id)
  confirmKick.value = null
}
</script>

<template>
  <div class="teacher" :class="{ floating }" aria-label="Kontrol pengajar" role="group">
    <button
      class="btn-secondary"
      :disabled="status !== 'open' || traveling"
      title="Pindahkan semua peserta ke titik kumpul"
      @click="gather"
    >
      Kumpulkan
    </button>
    <button
      type="button"
      class="btn-secondary switch"
      role="switch"
      :aria-checked="locked"
      :disabled="status !== 'open'"
      @click="setLocked(!locked)"
    >
      Kunci ruang
      <span class="state" aria-hidden="true">{{ locked ? 'Aktif' : 'Mati' }}</span>
    </button>
    <details class="more">
      <summary class="btn-secondary">
        Peserta &amp; chat
        <span class="state" aria-hidden="true">{{ participants.length }}</span>
      </summary>
      <div class="panel">
        <button
          type="button"
          class="btn-secondary switch wide"
          role="switch"
          :aria-checked="allChat"
          :disabled="toggling || status !== 'open'"
          @click="toggleAllChat"
        >
          Chat semua ruangan
          <span class="state" aria-hidden="true">{{ allChat ? 'Aktif' : 'Mati' }}</span>
        </button>
        <p v-if="toggleError" role="alert" class="toggle-error">{{ toggleError }}</p>
        <h3>Peserta ({{ participants.length }})</h3>
        <p v-if="!participants.length" class="room-meta">Belum ada peserta lain.</p>
        <ul class="people">
          <li v-for="p in participants" :key="p.id">
            <span
              >{{ p.name }}<template v-if="mutedIds.includes(p.id)"> (dibisukan)</template></span
            >
            <button
              type="button"
              class="btn-secondary"
              :aria-label="`${mutedIds.includes(p.id) ? 'Aktifkan chat' : 'Bisukan'} ${p.name}`"
              :disabled="status !== 'open'"
              @click="moderate(mutedIds.includes(p.id) ? 'unmute' : 'mute', p.id)"
            >
              {{ mutedIds.includes(p.id) ? 'Aktifkan chat' : 'Bisukan' }}
            </button>
            <button
              type="button"
              class="btn-secondary"
              :aria-label="`Keluarkan ${p.name} dari sesi`"
              :disabled="status !== 'open'"
              @click="confirmKick = p"
            >
              Keluarkan
            </button>
          </li>
        </ul>
        <div v-if="confirmKick" class="confirm" role="alertdialog" aria-labelledby="kick-q">
          <p id="kick-q">
            Keluarkan {{ confirmKick.name }}? Ia tidak bisa masuk lagi ke sesi ini sampai kamu
            izinkan.
          </p>
          <button type="button" class="btn-primary" @click="doKick">Ya, keluarkan</button>
          <button type="button" class="btn-secondary" @click="confirmKick = null">Batal</button>
        </div>
        <template v-if="banned.length">
          <h3>Dikeluarkan dari sesi ini</h3>
          <ul class="people">
            <li v-for="b in banned" :key="b.id">
              <span>{{ b.name }}</span>
              <button
                type="button"
                class="btn-secondary"
                :aria-label="`Izinkan ${b.name} masuk lagi`"
                @click="allow(b.id)"
              >
                Izinkan
              </button>
            </li>
          </ul>
        </template>
      </div>
    </details>
  </div>
</template>

<style scoped>
.teacher {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}
/* In the map rail: one line, never wraps (the rail has a fixed height). */
.floating {
  flex: none;
  flex-wrap: nowrap;
}
.floating .btn-secondary {
  min-height: var(--touch-min);
  white-space: nowrap;
}
.more summary {
  list-style: none;
  gap: var(--space-2);
}
.more summary::-webkit-details-marker {
  display: none;
}
.more[open] summary {
  border-color: var(--color-primary);
  color: var(--color-primary-strong);
}
.panel {
  display: grid;
  gap: var(--space-2);
  margin-top: var(--space-2);
}
.floating .more {
  position: relative;
}
.floating .panel {
  position: absolute;
  z-index: 3;
  top: calc(100% + var(--space-3));
  right: 0;
  width: min(22rem, 80vw);
  max-height: 60vh;
  overflow-y: auto;
  padding: var(--space-4);
  background: var(--color-surface);
  border: 2px solid var(--color-border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-raise);
}
.wide {
  width: 100%;
  justify-content: space-between;
}
h3 {
  margin: var(--space-2) 0 0;
  font-size: var(--text-md);
}
/* Setting name stays on the button; the pill shows its state. */
.switch {
  gap: var(--space-2);
}
.state {
  padding: 0 var(--space-2);
  border-radius: var(--radius-pill);
  background: var(--color-surface-alt);
  font-size: var(--text-sm);
}
.switch[aria-checked='true'] {
  border-color: var(--color-primary);
  color: var(--color-primary-strong);
}
.switch[aria-checked='true'] .state {
  background: var(--color-primary);
  color: var(--color-surface);
}
.people {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: var(--space-2);
}
.people li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}
.people li > span {
  flex-basis: 100%;
  font-weight: var(--weight-bold);
}
.confirm {
  display: grid;
  gap: var(--space-2);
  padding: var(--space-3);
  border: 2px solid var(--color-border);
  border-radius: var(--radius-md);
}
.confirm p,
.room-meta {
  margin: 0;
}
.room-meta {
  color: var(--color-ink-muted);
}
.toggle-error {
  margin: 0;
  font-size: var(--text-sm);
  font-weight: var(--weight-bold);
}
</style>
