<script setup lang="ts">
import type { MapData } from '@/realtime/mapdata'
import world from '@/assets/world/world.json'
// Each building takes its own colour from world.json so the minimap matches the labels on the map.
const groundColour = world.ground.color
const colourOf = (key: string) => world.places.find((p) => p.id === key)?.uiColor
defineProps<{ map: MapData; position: { x: number; y: number } | null }>()
</script>

<template>
  <svg class="minimap" :viewBox="`0 0 ${map.pixelWidth} ${map.pixelHeight}`" role="img" aria-label="Posisi kamu di kampus">
    <rect width="100%" height="100%" :fill="groundColour" />
    <rect v-for="building in map.buildings" :key="building.key"
      :x="building.x" :y="building.y" :width="building.w" :height="building.h" rx="20" class="building" :style="colourOf(building.key) ? { fill: colourOf(building.key) } : undefined" />
    <circle v-if="position" :cx="position.x" :cy="position.y" r="24" class="player" />
  </svg>
</template>

<style scoped>
.minimap {
  width: 160px;
  max-width: 30%;
  border: 2px solid var(--kv-surface);
  border-radius: var(--kv-radius-button);
  box-shadow: var(--kv-shadow-float);
}
.building { fill: var(--kv-map-lawn); stroke: var(--kv-ink); stroke-width: 4; }
.player { fill: var(--kv-primary); stroke: var(--kv-surface); stroke-width: 8; }
</style>
