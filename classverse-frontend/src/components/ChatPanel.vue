<script setup lang="ts">
import { computed, nextTick, ref, useId, useTemplateRef, watch } from 'vue'

import type { ChatState } from '@/realtime/chat'
import type { ChatScope } from '@/realtime/messages'

const props = defineProps<{
  chat: ChatState
  version: number
  error: string
  disabled: boolean
  campus: boolean
}>()
const emit = defineEmits<{ send: [text: string, scope: ChatScope] }>()

const MAX_LEN = 500
const tab = ref<ChatScope>('near')
const text = ref('')
const inputId = useId()
const log = useTemplateRef<HTMLDivElement>('log')

const tabs = computed(() => {
  void props.version
  const list: { scope: ChatScope; label: string }[] = []
  if (props.chat.zone.key) list.push({ scope: 'zone', label: props.chat.zone.name })
  list.push({ scope: 'near', label: 'Dekat' }, { scope: 'room', label: 'Semua' })
  return list
})
const lines = computed(() => {
  void props.version
  return [...props.chat.lines[tab.value]]
})
const zoneKey = computed(() => {
  void props.version
  return props.chat.zone.key
})
const roomOff = computed(() => {
  void props.version
  return tab.value === 'room' && !props.chat.allChatEnabled && !props.campus
})
const placeholder = computed(() => {
  if (roomOff.value) return 'Chat semua ruangan dimatikan pengajar'
  if (tab.value === 'zone') return `Tulis ke ${props.chat.zone.name}…`
  if (tab.value === 'near') return 'Tulis ke orang di dekatmu…'
  return 'Tulis ke semua orang di ruang…'
})
const remaining = computed(() => MAX_LEN - [...text.value].length)

// Entering a zone opens its tab; leaving it falls back to near.
watch(zoneKey, (key, prev) => {
  if (key && key !== prev) tab.value = 'zone'
  else if (!key && tab.value === 'zone') tab.value = 'near'
})
watch(
  () => lines.value.length,
  async () => {
    await nextTick()
    if (log.value) log.value.scrollTop = log.value.scrollHeight
  },
)

function send() {
  const t = text.value.trim()
  if (!t || props.disabled || roomOff.value || remaining.value < 0) return
  emit('send', t, tab.value)
  text.value = ''
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
    e.preventDefault()
    send()
  }
}

function time(iso: string) {
  return new Date(iso).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })
}
</script>

<template>
  <section class="chat" aria-label="Chat">
    <div class="tabs" role="tablist" aria-label="Tujuan chat">
      <button
        v-for="t in tabs"
        :id="`tab-${t.scope}`"
        :key="t.scope"
        role="tab"
        type="button"
        class="tab"
        :class="{ zone: t.scope === 'zone' }"
        :aria-selected="tab === t.scope"
        aria-controls="chat-log"
        @click="tab = t.scope"
      >
        {{ t.label }}
      </button>
    </div>
    <p v-if="zoneKey" class="zone-note">Kamu di {{ chat.zone.name }}</p>
    <div
      id="chat-log"
      ref="log"
      class="log"
      role="log"
      aria-live="polite"
      :aria-labelledby="`tab-${tab}`"
      tabindex="0"
    >
      <p v-if="!lines.length" class="empty">
        {{
          tab === 'near'
            ? 'Belum ada pesan. Orang dalam 5 tile darimu bisa membaca pesan di sini.'
            : tab === 'zone'
              ? 'Belum ada pesan di meja ini.'
              : 'Belum ada pesan untuk semua orang.'
        }}
      </p>
      <div v-for="l in lines" :key="l.key" class="line" :class="{ mine: l.mine }">
        <span class="who"
          >{{ l.mine ? 'Kamu' : l.fromName }}
          <span v-if="l.role !== 'participant' && !l.mine" class="role">Pengajar</span>
          <time :datetime="l.at">{{ time(l.at) }}</time></span
        >
        <p class="bubble">{{ l.text }}</p>
      </div>
    </div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <form class="compose" @submit.prevent="send">
      <label :for="inputId" class="sr-only">Pesan</label>
      <textarea
        :id="inputId"
        v-model="text"
        rows="1"
        :placeholder="placeholder"
        :disabled="disabled || roomOff"
        :aria-describedby="remaining < 50 ? `${inputId}-count` : undefined"
        @keydown="onKey"
      ></textarea>
      <button
        type="submit"
        class="send"
        aria-label="Kirim pesan"
        :disabled="disabled || roomOff || !text.trim() || remaining < 0"
      >
        <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
          <path
            d="M5 12h14M13 6l6 6-6 6"
            fill="none"
            stroke="currentColor"
            stroke-width="2.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
    </form>
    <p
      v-if="remaining < 50"
      :id="`${inputId}-count`"
      class="count"
      :class="{ over: remaining < 0 }"
    >
      Sisa {{ remaining }} karakter
    </p>
  </section>
</template>

<style scoped>
.chat {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
}
.tabs {
  display: flex;
  gap: var(--space-2);
}
.tab {
  flex: 1 1 0;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-height: var(--touch-min);
  padding: var(--space-2) var(--space-2);
  border: 2px solid transparent;
  border-radius: var(--radius-md);
  background: var(--color-surface-alt);
  color: var(--color-ink);
  font: inherit;
  font-weight: var(--weight-bold);
  cursor: pointer;
}
.tab[aria-selected='true'] {
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
  border-color: var(--color-primary);
}
.tab.zone[aria-selected='true'] {
  background: var(--color-accent-soft);
  color: var(--color-accent-ink);
  border-color: var(--color-accent);
}
.zone-note {
  margin: var(--space-2) 0 0;
  font-size: var(--text-sm);
  color: var(--color-accent-ink);
  font-weight: var(--weight-bold);
}
.log {
  flex: 1;
  min-height: 160px;
  overflow-y: auto;
  margin-top: var(--space-3);
  padding: var(--space-2) 0;
  border-top: 2px solid var(--color-border-soft);
}
.empty {
  color: var(--color-ink-muted);
  font-size: var(--text-md);
}
.line {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  margin-bottom: var(--space-3);
}
.line.mine {
  align-items: flex-end;
}
.who {
  font-size: var(--text-sm);
  color: var(--color-ink-muted);
  font-weight: var(--weight-bold);
  margin-bottom: var(--space-1);
  display: flex;
  gap: var(--space-2);
  align-items: center;
}
.who time {
  font-weight: var(--weight-regular);
}
.role {
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
  border-radius: var(--radius-pill);
  padding: 0 var(--space-2);
}
.bubble {
  margin: 0;
  max-width: 90%;
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-alt);
  border-radius: var(--radius-md);
  border-bottom-left-radius: var(--radius-xs);
  font-size: var(--text-md);
  line-height: var(--line-normal);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.mine .bubble {
  background: var(--color-primary);
  color: var(--color-surface);
  border-bottom-left-radius: var(--radius-md);
  border-bottom-right-radius: var(--radius-xs);
}
.error {
  margin: var(--space-2) 0 0;
  color: var(--color-ink);
  font-weight: var(--weight-bold);
  font-size: var(--text-sm);
}
.compose {
  display: flex;
  gap: var(--space-2);
  margin-top: var(--space-3);
  align-items: flex-end;
}
textarea {
  flex: 1;
  min-height: var(--touch-min);
  max-height: 120px;
  resize: vertical;
  box-sizing: border-box;
  padding: var(--space-2) var(--space-3);
  border: 2px solid var(--color-input-border);
  border-radius: var(--radius-md);
  font: inherit;
  font-size: var(--text-md);
  color: var(--color-ink);
  background: var(--color-surface);
}
textarea:focus-visible {
  outline: 3px solid var(--color-primary-strong);
  outline-offset: 1px;
}
.send {
  width: var(--touch-min);
  height: var(--touch-min);
  flex: none;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: var(--radius-md);
  background: var(--color-primary);
  color: var(--color-surface);
  cursor: pointer;
}
.send:disabled {
  background: var(--color-border);
  cursor: not-allowed;
}
.count {
  margin: var(--space-1) 0 0;
  font-size: var(--text-sm);
  color: var(--color-ink-muted);
  text-align: right;
}
.count.over {
  color: var(--color-ink);
  font-weight: var(--weight-bold);
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}
</style>
