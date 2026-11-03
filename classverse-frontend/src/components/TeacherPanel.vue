<script setup lang="ts">
import { computed, ref } from 'vue'

import { errorMessage } from '@/api/client'
import { roomApi } from '@/api/rooms'
import type { RoomHandle } from '@/realtime/useRoom'

// Teacher controls of a classroom/garden, shared by the map and mode HP.
const props = defineProps<{ room: RoomHandle }>()
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
  <div class="teacher">
    <button class="btn-secondary" :disabled="status !== 'open' || traveling" @click="gather">
      Kumpulkan peserta
    </button>
    <button
      type="button"
      class="btn-secondary toggle"
      role="switch"
      :aria-checked="allChat"
      :disabled="toggling || status !== 'open'"
      @click="toggleAllChat"
    >
      Chat semua ruangan: {{ allChat ? 'aktif' : 'mati' }}
    </button>
    <p v-if="toggleError" role="alert" class="toggle-error">{{ toggleError }}</p>
    <button
      type="button"
      class="btn-secondary toggle"
      role="switch"
      :aria-checked="locked"
      :disabled="status !== 'open'"
      @click="setLocked(!locked)"
    >
      Ruang: {{ locked ? 'terkunci' : 'terbuka' }}
    </button>
    <details class="moderation">
      <summary>Peserta ({{ participants.length }})</summary>
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
        <p class="room-meta">Dikeluarkan dari sesi ini</p>
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
    </details>
  </div>
</template>

<style scoped>
.moderation summary {
  min-height: var(--touch-min);
  display: flex;
  align-items: center;
  cursor: pointer;
  font-weight: var(--weight-bold);
}
.people {
  list-style: none;
  padding: 0;
  margin: 0 0 var(--space-3);
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
  margin-bottom: var(--space-3);
}
.room-meta {
  margin: 0 0 var(--space-3);
  color: var(--color-ink-muted);
}
.teacher {
  margin-bottom: var(--space-4);
}
.toggle {
  width: 100%;
}
.toggle-error {
  font-size: var(--text-sm);
  font-weight: var(--weight-bold);
}
</style>
