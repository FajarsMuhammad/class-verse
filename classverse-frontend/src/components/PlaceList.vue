<script setup lang="ts">
import type { Building } from '@/realtime/mapdata'
import type { BuildingCount } from '@/realtime/messages'

// PlaceList is the keyboard/screen-reader way to pick a campus building (the map is a canvas).
// Selecting opens the same place card as clicking the building on the map.
const props = defineProps<{ places: Building[]; selected?: string; counts: BuildingCount[] }>()
const emit = defineEmits<{ select: [key: string] }>()
const countOf = (key: string) => props.counts.find((c) => c.building_key === key)
</script>

<template>
  <section class="places" aria-labelledby="places-title">
    <h3 id="places-title">Gedung</h3>
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
  </section>
</template>

<style scoped>
.places { margin-bottom: var(--space-4); }
h3 { margin: 0 0 var(--space-2); font-size: var(--text-xs); font-weight: var(--weight-semibold); color: var(--color-ink-muted); text-transform: uppercase; letter-spacing: 0.04em; }
ul { margin: 0; padding: 0; list-style: none; display: grid; gap: 2px; }
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
</style>
