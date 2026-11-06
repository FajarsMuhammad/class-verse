<script setup lang="ts">
import { computed } from 'vue'

import type { ChatState } from '@/realtime/chat'
import type { VoiceHandle } from '@/realtime/useVoice'

// Mic control and teacher voice broadcast for a classroom/garden (slice 9).
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
const pending = computed(() => {
  void props.version
  return props.chat.voicePending.length
})
const channel = computed(() => {
  void props.version
  return props.chat.zone.key ? props.chat.zone.name : 'area umum'
})
const label = computed(() =>
  status.value === 'connecting'
    ? 'Menyambungkan suara…'
    : muted.value
      ? 'Mikrofon dimatikan pengajar'
      : micOn.value
        ? `Mikrofon menyala · terdengar di ${broadcasting.value ? 'seluruh ruang' : channel.value}`
        : `Mikrofon mati · kamu mendengar ${channel.value}`,
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
    <p class="state" role="status">{{ label }}</p>
    <button
      v-if="teacher"
      type="button"
      class="btn-secondary toggle"
      role="switch"
      :aria-checked="broadcasting"
      :disabled="disabled || status !== 'on'"
      @click="emit('broadcast', !broadcasting)"
    >
      Bicara ke semua ruang: {{ broadcasting ? 'aktif' : 'mati' }}
    </button>
    <button v-if="needsAudioStart" type="button" class="btn-primary toggle" @click="startAudio">
      Aktifkan suara
    </button>
    <p v-if="teacher && pending" role="status" class="error">
      Pemutusan suara {{ pending }} peserta belum terkonfirmasi. Server mencoba ulang; jangan anggap
      suara sudah terputus.
    </p>
    <p v-if="error" role="alert" class="error">{{ error }}</p>
  </div>
</template>

<style scoped>
.voice {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
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
.state {
  flex: 1;
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-ink-muted);
}
.toggle {
  width: 100%;
}
.error {
  margin: 0;
  font-size: var(--text-sm);
  font-weight: var(--weight-bold);
}
</style>
