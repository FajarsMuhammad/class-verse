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
      <div><h2>{{ building.name }}</h2><p>{{ building.description }}</p></div>
      <button class="btn-secondary" aria-label="Tutup detail tempat" @click="emit('close')">×</button>
    </header>
    <p v-if="building.staffOnly" class="restriction">Khusus pengajar dan admin</p>
    <p v-if="occupancy" class="occupancy">{{ occupancy.people }} orang · {{ occupancy.live_rooms }} ruang live</p>
    <p v-else class="occupancy">Jumlah peserta belum tersedia.</p>
    <button class="btn-primary action" data-testid="place-action" :disabled="disabled || restricted"
      @click="atDoor ? emit('rooms') : emit('walk')">{{ atDoor ? 'Pilih ruang' : 'Menuju pintu' }}</button>
    <!-- Modules are readable from anywhere on the map; staff-only buildings have none. -->
    <button v-if="!building.staffOnly" class="btn-secondary action" data-testid="place-modules" @click="emit('modules')">
      Lihat modul
    </button>
  </section>
</template>

<style scoped>
.place-card {
  box-sizing: border-box;
  width: min(320px, calc(100% - var(--space-6)));
  padding: var(--space-4);
  border: 1px solid var(--kv-line);
  border-radius: var(--kv-radius-card);
  background: var(--kv-surface);
  color: var(--kv-ink);
  box-shadow: var(--kv-shadow-float);
}
header { display: flex; align-items: flex-start; gap: var(--space-2); justify-content: space-between; }
h2 { font: var(--weight-semibold) var(--text-lg)/var(--line-tight) var(--kv-font-display); margin: 0; }
p { margin: var(--space-2) 0; font-size: var(--text-sm); color: var(--kv-text-muted); }
header button { padding: 0; min-width: var(--touch-min); flex: none; }
.restriction { color: var(--kv-primary-soft-text); }
.action { width: 100%; margin-top: var(--space-2); }
</style>
