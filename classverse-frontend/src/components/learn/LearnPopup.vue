<script setup lang="ts">
import { onBeforeUnmount, onMounted, useTemplateRef } from 'vue'

// Shared popup for learning (class tasks and building modules): a large native modal over the map.
// showModal() gives the focus trap, Esc and the top layer; the map stops moving because CampusScene only
// reads WASD while its canvas has focus. Children render their own header and leave room for the close button.
defineProps<{ labelledby: string }>()
const emit = defineEmits<{ close: [] }>()
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
let opener: HTMLElement | null = null

onMounted(() => {
  opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
  dialog.value?.showModal()
})
// Focus goes back to whatever opened the popup (a task card, a building's "Lihat modul").
onBeforeUnmount(() => { if (opener?.isConnected) opener.focus() })

function close() {
  dialog.value?.close()
}
// Only a click on the backdrop lands on the <dialog> itself; the box fills its whole padding-free area.
function onClick(e: MouseEvent) {
  if (e.target === dialog.value) close()
}
defineExpose({ close })
</script>

<template>
  <Teleport to="body">
    <dialog ref="dialog" class="learn-popup" :aria-labelledby="labelledby" @close="emit('close')" @click="onClick">
      <div class="learn-box">
        <button type="button" class="learn-close" aria-label="Tutup" @click="close">
          <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path d="M18 6 6 18M6 6l12 12" /></svg>
        </button>
        <slot />
      </div>
    </dialog>
  </Teleport>
</template>

<style scoped>
.learn-popup {
  box-sizing: border-box;
  width: min(1160px, 100vw - var(--space-6) * 2);
  max-width: none;
  height: min(790px, 100dvh - var(--space-6) * 2);
  max-height: none;
  padding: 0;
  border: 0;
  border-radius: var(--radius-2xl);
  background: var(--color-surface);
  color: var(--color-ink);
  font-family: var(--font-body);
  box-shadow: 0 24px 60px color-mix(in srgb, var(--kv-ink) 35%, transparent);
  overflow: hidden;
}
.learn-popup::backdrop {
  background: color-mix(in srgb, var(--kv-ink) 58%, transparent);
}
.learn-box {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
}
.learn-close {
  position: absolute;
  z-index: 1;
  top: var(--space-5);
  right: var(--space-6);
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border: 1px solid var(--kv-line);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
  color: var(--color-ink);
  cursor: pointer;
}
.learn-close svg { fill: none; stroke: currentColor; stroke-width: 2.4; stroke-linecap: round; }
.learn-close:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
@media (max-width: 600px) {
  .learn-popup { width: 100vw; height: 100dvh; border-radius: 0; }
  .learn-close { top: var(--space-3); right: var(--space-3); }
}
</style>
