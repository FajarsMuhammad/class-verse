<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from 'vue'
import type { EmoteKey } from '@/realtime/messages'
const props = defineProps<{ disabled: boolean; send: (key: EmoteKey) => boolean }>()
const options: { key: EmoteKey; label: string }[] = [
  { key: 'wave', label: 'Halo' }, { key: 'hand', label: 'Angkat tangan' },
  { key: 'quiz', label: 'Ayo kuis' }, { key: 'cheer', label: 'Hore' },
]
const open = ref(false)
const cooling = ref(false)
const announcement = ref('')
const toggle = ref<HTMLButtonElement | null>(null)
const menu = ref<HTMLElement | null>(null)
let timer: ReturnType<typeof setTimeout> | undefined
async function flip() {
  open.value = !open.value
  if (open.value) {
    await nextTick()
    menu.value?.querySelector('button')?.focus()
  }
}
function close() {
  open.value = false
  toggle.value?.focus()
}
function pick(key: EmoteKey, label: string) {
  if (props.disabled || cooling.value || !props.send(key)) return
  cooling.value = true
  announcement.value = `Emote ${label} dikirim`
  close()
  timer = setTimeout(() => { cooling.value = false }, 1000)
}
onBeforeUnmount(() => clearTimeout(timer))
</script>
<template>
  <div class="emote" @keydown.esc="close">
    <div v-if="open && !disabled" id="emote-menu" ref="menu" class="menu" role="group" aria-label="Pilih emote">
      <button v-for="option in options" :key="option.key" type="button"
        :disabled="cooling" @click="pick(option.key, option.label)">{{ option.label }}</button>
    </div>
    <button ref="toggle" type="button" class="toggle" aria-controls="emote-menu"
      :aria-expanded="open && !disabled" :disabled="disabled || cooling" @click="flip">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"
        stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="12" cy="12" r="9" /><path d="M8 14s1.5 2 4 2 4-2 4-2" /><path d="M9 9h.01M15 9h.01" />
      </svg>
      Emote
    </button>
    <span class="sr-only" role="status">{{ announcement }}</span>
  </div>
</template>
<style scoped>
.emote { position: relative; flex: none; }
.toggle {
  display: flex; align-items: center; gap: var(--space-2);
  min-height: var(--touch-min); padding: 0 var(--space-4);
  border: none; border-radius: var(--radius-pill);
  background: var(--color-primary); color: var(--color-surface);
  font: inherit; font-weight: var(--weight-semibold); cursor: pointer;
}
.menu {
  position: absolute; left: 0; bottom: calc(100% + var(--space-3)); z-index: 3;
  display: flex; flex-wrap: wrap; gap: var(--space-2); padding: var(--space-2);
  width: max-content; max-width: calc(100vw - 4 * var(--space-4)); box-sizing: border-box;
  background: var(--color-surface); border: 1px solid var(--color-border);
  border-radius: var(--radius-lg); box-shadow: var(--shadow-raise); white-space: nowrap;
}
.menu button {
  min-height: var(--touch-min); padding: 0 var(--space-3);
  border: none; border-radius: var(--radius-md);
  background: var(--color-accent-soft); color: var(--color-ink);
  font: inherit; font-weight: var(--weight-semibold); cursor: pointer;
}
button:disabled { opacity: 0.5; cursor: default; }
button:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
</style>
