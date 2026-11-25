<script setup lang="ts">
import { nextTick, ref, useTemplateRef } from 'vue'
import type { Building } from '@/realtime/mapdata'
import type { BuildingCount } from '@/realtime/messages'

// PlaceList is the keyboard/screen-reader way to pick a campus building (the map is a canvas). It sits over the
// map as a small button so the panel and the map stay clear; picking opens the same place card as the map does.
const props = defineProps<{ places: Building[]; selected?: string; counts: BuildingCount[] }>()
const emit = defineEmits<{ select: [key: string] }>()
const countOf = (key: string) => props.counts.find((c) => c.building_key === key)
const open = ref(false)
const trigger = useTemplateRef<HTMLButtonElement>('trigger')
async function close() {
  open.value = false
  await nextTick()
  trigger.value?.focus()
}
function pick(key: string) {
  emit('select', key)
  void close()
}
</script>

<template>
  <div class="places" @keydown.esc="close">
    <button ref="trigger" type="button" class="trigger" :aria-expanded="open" @click="open = !open">
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path d="M3 5l6-2 6 2 6-2v16l-6 2-6-2-6 2zM9 3v16M15 5v16" /></svg>
      Gedung
    </button>
    <div v-if="open" class="sheet" role="group" aria-label="Pilih gedung">
      <ul>
        <li v-for="b in places" :key="b.key">
          <button type="button" :aria-pressed="selected === b.key" @click="pick(b.key)">
            <span class="name">{{ b.name }}</span>
            <span class="meta">
              {{ b.staffOnly ? 'Khusus pengajar' : b.description }}<template v-if="countOf(b.key)">
                · {{ countOf(b.key)!.people }} orang<template v-if="countOf(b.key)!.live_rooms"> · {{ countOf(b.key)!.live_rooms }} live</template></template>
            </span>
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.places { position: relative; }
.trigger {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  min-height: var(--touch-min);
  padding: 0 var(--space-4);
  border: 1px solid var(--kv-line);
  border-radius: var(--kv-radius-button);
  background: var(--kv-surface);
  color: var(--color-ink);
  font: inherit;
  font-size: var(--text-sm);
  font-weight: var(--weight-bold);
  box-shadow: var(--kv-shadow-float);
  cursor: pointer;
}
.trigger svg { fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linejoin: round; }
.trigger:focus-visible, .sheet button:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
.sheet {
  position: absolute;
  z-index: 1;
  bottom: calc(100% + var(--space-2));
  left: 0;
  width: min(280px, calc(100vw - var(--space-6)));
  max-height: min(60dvh, 420px);
  overflow: auto;
  padding: var(--space-2);
  border: 1px solid var(--kv-line);
  border-radius: var(--radius-lg);
  background: var(--kv-surface);
  box-shadow: var(--kv-shadow-popover);
}
ul { margin: 0; padding: 0; list-style: none; }
.sheet button {
  display: grid;
  gap: 2px;
  width: 100%;
  min-height: var(--touch-min);
  padding: var(--space-2) var(--space-3);
  border: 0;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-ink);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.sheet button:hover { background: var(--color-surface-alt); }
.sheet button[aria-pressed='true'] { background: var(--color-primary-soft); }
.name { font-size: var(--text-md); font-weight: var(--weight-semibold); overflow-wrap: anywhere; }
.meta { color: var(--kv-text-muted); font-size: var(--text-sm); }
</style>
