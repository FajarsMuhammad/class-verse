<script setup lang="ts">
import { computed, nextTick, ref, useId, useTemplateRef, watch } from 'vue'

import type { ChatLine, ChatState } from '@/realtime/chat'
import type { ChatScope } from '@/realtime/messages'

const props = defineProps<{
  chat: ChatState
  version: number
  error: string
  disabled: boolean
  campus: boolean
  teacher?: boolean
  guest?: boolean
  // Presence lives in the chat (map and panels show no online counts). Omit both to hide it.
  online?: number
  people?: string[]
}>()
const emit = defineEmits<{
  send: [text: string, scope: ChatScope]
  report: [line: ChatLine]
}>()

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
  if (props.teacher) list.push({ scope: 'broadcast', label: 'Pengumuman' })
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
const muted = computed(() => {
  void props.version
  return props.chat.muted && !props.campus
})
// roomOff blocks sending: muted by the teacher, or "Semua" switched off.
const roomOff = computed(() => {
  void props.version
  return muted.value || (tab.value === 'room' && !props.chat.allChatEnabled && !props.campus)
})
const placeholder = computed(() => {
  if (muted.value) return 'Kamu dibisukan pengajar'
  if (roomOff.value) return 'Chat semua ruangan dimatikan pengajar'
  if (tab.value === 'zone') return `Tulis ke ${props.chat.zone.name}…`
  if (tab.value === 'near') return 'Tulis ke orang di dekatmu…'
  if (tab.value === 'broadcast') return 'Tulis pengumuman untuk seluruh ruang…'
  return 'Tulis ke semua orang di ruang…'
})
const remaining = computed(() => MAX_LEN - [...text.value].length)
const announcement = computed(() => {
  void props.version
  const lines = props.chat.lines.broadcast
  return lines[lines.length - 1]
})

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
    <details v-if="online !== undefined" class="presence">
      <summary>
        <span class="dot" aria-hidden="true"></span>{{ online }} orang online
        <svg class="chev" viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M6 9l6 6 6-6" /></svg>
      </summary>
      <ul>
        <li v-for="(name, i) in people ?? []" :key="i">{{ name }}</li>
      </ul>
    </details>
    <div class="tabs" role="tablist" aria-label="Tujuan chat">
      <button
        v-for="t in tabs"
        :id="`tab-${t.scope}`"
        :key="t.scope"
        role="tab"
        type="button"
        class="tab"
        :class="{ zone: t.scope === 'zone', broadcast: t.scope === 'broadcast' }"
        :aria-selected="tab === t.scope"
        aria-controls="chat-log"
        @click="tab = t.scope"
      >
        {{ t.label }}
      </button>
    </div>
    <p v-if="announcement" class="zone-note" role="status">Pengumuman: {{ announcement.text }}</p>
    <p v-if="muted" class="zone-note" role="status">
      Pengajar membisukan chatmu. Kamu tetap bisa membaca pesan.
    </p>
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
            ? campus
              ? 'Belum ada pesan. Orang di dalam lingkaran di sekitarmu bisa membaca pesan di sini.'
              : 'Belum ada pesan. Orang di dalam lingkaran, sekitar satu meja, bisa membaca pesan di sini.'
            : tab === 'zone'
              ? 'Belum ada pesan di meja ini.'
              : 'Belum ada pesan untuk semua orang.'
        }}
      </p>
      <div v-for="l in lines" :key="l.key" class="line" :class="{ mine: l.mine }">
        <span class="who"
          >{{ l.mine ? 'Kamu' : l.fromName }}
          <span v-if="l.role === 'guest'" class="role">Tamu</span>
                <span v-else-if="l.role !== 'participant' && !l.mine" class="role">Pengajar</span>
          <time :datetime="l.at">{{ time(l.at) }}</time></span
        >
        <button
          v-if="!guest && !l.mine"
          type="button"
          class="report"
          :aria-label="`Laporkan pesan dari ${l.fromName}`"
          @click="emit('report', l)"
        >
          Laporkan
        </button>
        <p class="bubble"><strong v-if="l.broadcast">Pengumuman · </strong>{{ l.text }}</p>
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
        <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
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
/* Compact controls (docs/visual.md "Kontrol ringkas", reff-design-4): --control-h buttons,
   --control-sm ghost/secondary, 10px radius, 1px lines. */
.chat {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  min-height: 0;
  height: 100%;
}
svg { fill: none; stroke: currentColor; stroke-width: 2.5; stroke-linecap: round; stroke-linejoin: round; flex: none; }
.presence { border: 1px solid var(--kv-line); border-radius: var(--control-radius); background: var(--kv-surface); }
.presence summary {
  display: flex; align-items: center; gap: var(--space-2); height: var(--control-sm); padding: 0 var(--space-3);
  font-size: var(--text-sm); font-weight: var(--weight-semibold); cursor: pointer; list-style: none;
}
.presence summary::-webkit-details-marker { display: none; }
.presence summary:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 1px; border-radius: var(--control-radius); }
.presence .chev { margin-left: auto; stroke-width: 2; color: var(--kv-text-muted); transition: transform 0.15s; }
.presence[open] .chev { transform: rotate(180deg); }
.dot { width: 8px; height: 8px; border-radius: 50%; background: var(--color-speaking); flex: none; }
.presence ul {
  margin: 0; padding: 0 var(--space-3) var(--space-2); list-style: none; max-height: 160px; overflow-y: auto;
  font-size: var(--text-sm); color: var(--kv-ink);
}
.presence li { padding: 2px 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
/* Segmented control: one track, the active tab is a raised white chip. */
.tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  padding: 2px;
  border-radius: var(--control-radius);
  background: var(--color-surface-alt);
}
.tab.broadcast {
  flex-basis: 100%;
}
.tab {
  flex: 1 1 0;
  min-width: 0;
  height: var(--control-sm);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 0 var(--space-2);
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--kv-text-muted);
  font: inherit;
  font-size: var(--text-sm);
  font-weight: var(--weight-semibold);
  cursor: pointer;
}
.tab:hover { color: var(--kv-ink); }
.tab[aria-selected='true'] {
  background: var(--kv-surface);
  color: var(--color-primary-strong);
  box-shadow: 0 1px 3px rgba(29, 33, 64, 0.12);
}
.tab.zone[aria-selected='true'] {
  color: var(--color-accent-ink);
}
.tab:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: -3px; }
.zone-note {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-accent-ink);
  font-weight: var(--weight-semibold);
}
.log {
  flex: 1;
  min-height: 160px;
  overflow-y: auto;
  padding: var(--space-2) 0;
}
.empty {
  margin: var(--space-4) var(--space-2);
  color: var(--kv-text-muted);
  font-size: var(--text-sm);
  text-align: center;
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
  font-size: var(--text-xs);
  color: var(--kv-text-muted);
  font-weight: var(--weight-semibold);
  margin-bottom: 2px;
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
  border-radius: 12px;
  border-bottom-left-radius: 4px;
  font-size: var(--text-sm);
  line-height: var(--line-normal);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.report {
  order: 3;
  height: 24px;
  padding: 0 var(--space-1);
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--kv-text-muted);
  font: inherit;
  font-size: var(--text-xs);
  cursor: pointer;
}
.report:hover { background: var(--color-surface-alt); color: var(--kv-ink); }
.report:focus-visible {
  outline: 3px solid var(--color-primary-strong);
}
.mine .bubble {
  background: var(--color-primary);
  color: var(--color-surface);
  border-bottom-left-radius: 12px;
  border-bottom-right-radius: 4px;
}
.error {
  margin: 0;
  color: var(--color-ink);
  font-weight: var(--weight-bold);
  font-size: var(--text-sm);
}
/* Composer: one bordered field, the send button sits inside it. */
.compose {
  display: flex;
  gap: var(--space-1);
  align-items: flex-end;
  padding: 3px;
  border: 1px solid var(--color-input-border);
  border-radius: 12px;
  background: var(--kv-surface);
}
.compose:focus-within { border-color: var(--color-primary); box-shadow: 0 0 0 3px var(--color-primary-soft); }
textarea {
  flex: 1;
  min-height: var(--control-h);
  max-height: 120px;
  resize: none;
  box-sizing: border-box;
  padding: var(--space-2);
  border: 0;
  background: transparent;
  font: inherit;
  font-size: var(--text-sm);
  color: var(--color-ink);
}
textarea:focus-visible { outline: none; }
.send {
  width: var(--control-h);
  height: var(--control-h);
  flex: none;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: var(--control-radius);
  background: var(--color-primary);
  color: var(--color-surface);
  cursor: pointer;
}
.send:hover:not(:disabled) { background: var(--color-primary-strong); }
.send:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
.send:disabled {
  background: var(--color-border);
  cursor: not-allowed;
}
.count {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--kv-text-muted);
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
@media (prefers-reduced-motion: reduce) { .presence .chev { transition: none; } }
</style>
