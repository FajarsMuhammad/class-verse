<script setup lang="ts">
import type { Building } from '@/realtime/mapdata'
import type { BuildingCount } from '@/realtime/messages'

// PlaceList is the keyboard/screen-reader way to pick a campus building (the map is a canvas).
// Selecting opens the same place card as clicking the building on the map.
const props = defineProps<{ places: Building[]; selected?: string; counts: BuildingCount[] }>()
const emit = defineEmits<{ select: [key: string] }>()
const countOf = (key: string) => props.counts.find((c) => c.building_key === key)
// Open by default; collapsing is remembered so the panel stays short for mouse users.
const OPEN_KEY = 'kv-places-open'
const open = localStorage.getItem(OPEN_KEY) !== '0'
function remember(e: Event) {
  localStorage.setItem(OPEN_KEY, (e.target as HTMLDetailsElement).open ? '1' : '0')
}
</script>

<template>
  <details class="places" :open="open" @toggle="remember">
    <summary>
      <span>Gedung ({{ places.length }})</span>
      <svg class="chev" viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M6 15l6-6 6 6" /></svg>
    </summary>
    <ul>
      <li v-for="b in places" :key="b.key">
        <button type="button" :aria-pressed="selected === b.key" @click="emit('select', b.key)">
          <span class="name">{{ b.name }}</span>
          <span class="meta">
            {{ b.staffOnly ? 'Khusus pengajar' : b.description }}<template v-if="countOf(b.key)">
              · {{ countOf(b.key)!.people }} orang<template v-if="countOf(b.key)!.live_rooms"> · {{ countOf(b.key)!.live_rooms }} live</template></template>
          </span>
        </button>
      </li>
    </ul>
  </details>
</template>

<style scoped>
/* Docked at the bottom of the side panel: pushed down when the panel is short (margin-top: auto),
   pinned to its bottom edge while the panel content scrolls (sticky). The list scrolls on its own. */
.places {
  position: sticky;
  bottom: 0;
  z-index: 1;
  margin-top: auto;
  padding-top: var(--space-2);
  border-top: 1px solid var(--color-border-soft);
  background: var(--color-surface);
}
summary {
  display: flex; align-items: center; justify-content: space-between;
  min-height: var(--touch-min); padding: 0 var(--space-2); border-radius: 10px;
  font-size: var(--text-sm); font-weight: var(--weight-semibold); color: var(--color-ink);
  cursor: pointer; list-style: none;
}
summary::-webkit-details-marker { display: none; }
summary:hover { background: var(--color-surface-alt); }
summary:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: -3px; }
.chev { fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; transform: rotate(180deg); transition: transform 0.15s; }
.places[open] .chev { transform: none; }
ul { margin: 0; padding: 0; list-style: none; display: grid; gap: 2px; max-height: min(40vh, 320px); overflow-y: auto; }
button {
  display: grid; width: 100%; min-height: var(--touch-min); padding: var(--space-1) var(--space-2);
  border: 0; border-radius: 10px; background: transparent; color: var(--color-ink);
  font: inherit; text-align: left; cursor: pointer;
}
button:hover { background: var(--color-surface-alt); }
button[aria-pressed='true'] { background: var(--color-primary-soft); color: var(--color-primary-strong); }
button:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: -3px; }
.name { font-size: var(--text-sm); font-weight: var(--weight-semibold); }
.meta { font-size: var(--text-xs); color: var(--color-ink-muted); }
@media (prefers-reduced-motion: reduce) { .chev { transition: none; } }
</style>
