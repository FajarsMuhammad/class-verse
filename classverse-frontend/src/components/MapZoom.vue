<script setup lang="ts">
const zoom = defineModel<number>({ default: 0.5 })
function change(delta: number) {
  zoom.value = Math.max(0.3, Math.min(1, Math.round((zoom.value + delta) * 10) / 10))
}
</script>

<template>
  <div class="zoom" role="group" aria-label="Zoom peta">
    <button aria-label="Perbesar peta" :disabled="zoom >= 1" @click="change(0.1)">+</button>
    <button aria-label="Reset zoom peta" @click="zoom = 0.5">{{ Math.round(zoom * 100) }}%</button>
    <button aria-label="Perkecil peta" :disabled="zoom <= 0.3" @click="change(-0.1)">−</button>
  </div>
</template>

<style scoped>
.zoom {
  display: flex;
  background: var(--kv-surface);
  border-radius: var(--kv-radius-button);
  box-shadow: var(--kv-shadow-float);
  overflow: hidden;
}
button {
  min-width: var(--touch-min);
  min-height: var(--touch-min);
  border: 0;
  background: transparent;
  color: var(--kv-ink);
  font: inherit;
  cursor: pointer;
}
button:disabled { opacity: 0.4; cursor: default; }
button:focus-visible { outline: 2px solid var(--kv-primary); outline-offset: -3px; }
</style>
