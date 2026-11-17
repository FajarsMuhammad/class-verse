<script setup lang="ts">
import { computed } from 'vue'
import type { Building } from '@/realtime/mapdata'
import type { BuildingCount } from '@/realtime/messages'
const props = defineProps<{ building: Building; occupancy?: BuildingCount; atDoor: boolean; teacher: boolean; disabled: boolean }>()
const emit = defineEmits<{ close: []; walk: []; rooms: []; modules: [] }>()
const restricted = computed(() => props.building.staffOnly && !props.teacher)
</script>

<template>
  <section class="place-card" :aria-label="`Detail ${building.name}`">
    <header>
      <span class="badge" aria-hidden="true">
        <svg viewBox="0 0 24 24" width="16" height="16"><path d="M4 21V9l8-5 8 5v12h-5v-6H9v6z" /></svg>
      </span>
      <div class="title"><h2>{{ building.name }}</h2><p>{{ building.description }}</p></div>
      <button class="close" aria-label="Tutup detail tempat" @click="emit('close')">
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18" /></svg>
      </button>
    </header>
    <p v-if="building.staffOnly" class="restriction">Khusus pengajar dan admin</p>
    <p v-if="occupancy" class="occupancy">{{ occupancy.people }} orang · {{ occupancy.live_rooms }} ruang live</p>
    <p v-else class="occupancy">Jumlah peserta belum tersedia.</p>
    <div class="actions">
      <button class="primary" data-testid="place-action" :disabled="disabled || restricted"
        @click="atDoor ? emit('rooms') : emit('walk')">
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
          <path v-if="atDoor" d="M4 21V4h11v17M15 12h0M15 4l5 2v15h-5" /><path v-else d="M5 12h14M13 6l6 6-6 6" />
        </svg>
        {{ atDoor ? 'Pilih ruang' : 'Menuju pintu' }}
      </button>
      <!-- Modules are readable from anywhere on the map; staff-only buildings have none. -->
      <button v-if="!building.staffOnly" class="icon" data-testid="place-modules" aria-label="Lihat modul"
        title="Lihat modul" @click="emit('modules')">
        <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path d="M4 5h6a2 2 0 0 1 2 2v13a2 2 0 0 0-2-2H4zM20 5h-6a2 2 0 0 0-2 2v13a2 2 0 0 1 2-2h6z" /></svg>
      </button>
    </div>
  </section>
</template>

<style scoped>
/* Compact floating card (docs/screens/reff-design-4.png): icon + title, ghost close,
   one wide primary action and a square icon action of the same height. */
.place-card {
  box-sizing: border-box;
  width: min(300px, calc(100% - var(--space-6)));
  padding: var(--space-3);
  border: 1px solid var(--kv-line);
  border-radius: var(--kv-radius-card);
  background: var(--kv-surface);
  color: var(--kv-ink);
  box-shadow: var(--kv-shadow-float);
}
svg { fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; flex: none; }
header { display: flex; align-items: flex-start; gap: var(--space-2); }
.badge {
  display: grid; place-items: center; flex: none; width: 28px; height: 28px; border-radius: 8px;
  background: var(--color-primary-soft); color: var(--color-primary-strong);
}
.title { flex: 1; min-width: 0; }
h2 { margin: 0; font: var(--weight-semibold) var(--text-sm)/1.3 var(--kv-font-display); }
p { margin: 0; font-size: var(--text-xs); line-height: 1.4; color: var(--kv-text-muted); }
.restriction, .occupancy { margin: var(--space-2) 0 0 calc(28px + var(--space-2)); }
.restriction { color: var(--kv-primary-soft-text); }
.close {
  display: grid; place-items: center; flex: none; width: var(--control-sm); height: var(--control-sm); margin: -4px -4px 0 0;
  border: 0; border-radius: 8px; background: transparent; color: var(--kv-text-muted); cursor: pointer;
}
.close:hover { background: var(--color-primary-soft); color: var(--kv-ink); }
.actions { display: flex; gap: var(--space-2); margin-top: var(--space-3); }
.primary, .icon {
  display: inline-flex; align-items: center; justify-content: center; gap: var(--space-2);
  height: var(--control-h); border-radius: var(--control-radius); font: var(--weight-semibold) var(--text-sm)/1 var(--font-body); cursor: pointer;
}
.primary { flex: 1; min-width: 0; padding: 0 var(--space-3); border: 0; background: var(--color-primary); color: var(--color-surface); }
.primary:hover:not(:disabled) { background: var(--color-primary-strong); }
.primary:disabled { opacity: 0.6; cursor: not-allowed; }
.icon { flex: none; width: var(--control-h); padding: 0; border: 1px solid var(--kv-line); background: var(--kv-surface); color: var(--kv-ink); }
.icon:hover { background: var(--color-primary-soft); }
.close:focus-visible, .primary:focus-visible, .icon:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
</style>
