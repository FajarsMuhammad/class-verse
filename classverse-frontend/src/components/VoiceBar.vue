<script setup lang="ts">
import { computed } from 'vue'

import type { ChatState } from '@/realtime/chat'
import type { VoiceHandle } from '@/realtime/useVoice'

// Mic, channel chip and teacher broadcast in the room's bottom rail (slice 9).
// The rail has a fixed height: nothing here may wrap or grow, or the map would
// jump on every table crossing. Messages float above the rail instead.
const props = defineProps<{
  voice: VoiceHandle
  chat: ChatState
  version: number
  selfId: string | null
  teacher: boolean
  disabled: boolean
}>()
const emit = defineEmits<{ broadcast: [on: boolean] }>()
const { status, error, micOn, needsAudioStart, toggleMic, startAudio } = props.voice

const muted = computed(() => {
  void props.version
  return props.chat.muted
})
const broadcasting = computed(() => {
  void props.version
  return !!props.selfId && props.chat.voiceBroadcast.includes(props.selfId)
})
// Other people's unconfirmed evictions. Your own is just "Menyambungkan suara".
const pending = computed(() => {
  void props.version
  return props.chat.voicePending.filter((id) => id !== props.selfId).length
})
const zone = computed(() => {
  void props.version
  return props.chat.zone
})
const channel = computed(() =>
  broadcasting.value ? 'Seluruh ruang' : zone.value.key ? zone.value.name : 'Area umum',
)
const state = computed(() =>
  status.value === 'connecting'
    ? 'menyambungkan…'
    : muted.value
      ? 'dimatikan pengajar'
      : micOn.value
        ? 'kamu bicara'
        : 'mikrofon mati',
)
// Spoken by screen readers and used as tooltip; the chip shows the short form.
const label = computed(() =>
  status.value === 'connecting'
    ? 'Menyambungkan suara…'
    : `Mendengar ${channel.value}, ${state.value}`,
)
</script>

<template>
  <div v-if="status !== 'off' && status !== 'unavailable'" class="voice" aria-label="Suara">
    <button
      type="button"
      class="mic"
      :class="{ on: micOn }"
      :aria-pressed="micOn"
      :aria-label="micOn ? 'Matikan mikrofon' : 'Nyalakan mikrofon'"
      :disabled="disabled || status !== 'on' || muted"
      @click="toggleMic"
    >
      <svg viewBox="0 0 24 24" width="24" height="24" aria-hidden="true">
        <rect
          x="9"
          y="3"
          width="6"
          height="11"
          rx="3"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        />
        <path
          d="M5 11a7 7 0 0 0 14 0M12 18v3"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
        />
        <path
          v-if="!micOn"
          d="M4 4l16 16"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
        />
      </svg>
    </button>
    <p
      class="channel"
      :class="{ zone: !!zone.key && !broadcasting, wait: status === 'connecting' }"
      role="status"
      :title="label"
    >
      <span class="sr-only">{{ label }}</span>
      <strong aria-hidden="true">{{ channel }}</strong>
      <span aria-hidden="true">{{ state }}</span>
    </p>
    <button
      v-if="teacher"
      type="button"
      class="btn-secondary switch"
      role="switch"
      :aria-checked="broadcasting"
      :disabled="disabled || status !== 'on'"
      @click="emit('broadcast', !broadcasting)"
    >
      Bicara ke semua ruang
      <span class="pill" aria-hidden="true">{{ broadcasting ? 'Aktif' : 'Mati' }}</span>
    </button>
    <div class="notes">
      <button v-if="needsAudioStart" type="button" class="btn-primary" @click="startAudio">
        Aktifkan suara
      </button>
      <p v-if="teacher && pending" role="status" class="note">
        Pemutusan suara {{ pending }} peserta belum terkonfirmasi. Server mencoba ulang; jangan
        anggap suara sudah terputus.
      </p>
      <p v-if="error && status !== 'connecting'" role="alert" class="note">{{ error }}</p>
    </div>
  </div>
</template>

<style scoped>
.voice {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-width: 0;
}
.mic {
  width: 52px;
  height: 52px;
  flex: none;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: var(--radius-pill);
  background: var(--color-ink);
  color: var(--color-surface);
  cursor: pointer;
}
.mic.on {
  background: var(--color-speaking);
  color: var(--color-ink);
}
.mic:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.mic:focus-visible {
  outline: 3px solid var(--color-primary-strong);
  outline-offset: 2px;
}
/* The channel chip uses the same Mangga as the highlighted table: the table
   you stand at is the table you hear. Fixed width, so state changes never
   move anything. */
.channel {
  flex: none;
  width: 15rem;
  min-height: var(--touch-min);
  box-sizing: border-box;
  display: flex;
  align-items: baseline;
  gap: var(--space-2);
  margin: 0;
  padding: var(--space-2) var(--space-4);
  border: 1px solid var(--kv-line);
  border-radius: var(--radius-pill);
  background: var(--color-surface-alt);
  white-space: nowrap;
  overflow: hidden;
}
.channel strong {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: var(--text-md);
}
.channel span[aria-hidden] {
  flex: none;
  font-size: var(--text-sm);
  color: var(--color-ink-muted);
}
.channel.zone {
  border-color: var(--color-accent);
  background: var(--color-accent-soft);
  color: var(--color-accent-ink);
}
.channel.zone span[aria-hidden] {
  color: var(--color-accent-ink);
}
.channel.wait {
  border-style: dashed;
}
.switch {
  flex: none;
  gap: var(--space-2);
  min-height: var(--touch-min);
}
.pill {
  padding: 0 var(--space-2);
  border-radius: var(--radius-pill);
  background: var(--color-surface-alt);
  font-size: var(--text-sm);
}
.switch[aria-checked='true'] {
  border-color: var(--color-primary);
  color: var(--color-primary-strong);
}
.switch[aria-checked='true'] .pill {
  background: var(--color-primary);
  color: var(--color-surface);
}
/* Notes float above the rail instead of growing it. */
.notes {
  position: absolute;
  left: 0;
  bottom: calc(100% + var(--space-2));
  display: grid;
  gap: var(--space-2);
  max-width: min(32rem, 100%);
}
.note {
  margin: 0;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  background: var(--color-accent-soft);
  color: var(--color-accent-ink);
  font-size: var(--text-sm);
  font-weight: var(--weight-bold);
  box-shadow: var(--shadow-raise);
}
</style>
