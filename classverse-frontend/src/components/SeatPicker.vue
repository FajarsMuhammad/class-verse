<script setup lang="ts">
import type { Seat } from '@/realtime/mapdata'
const props = defineProps<{ seats: Seat[]; self: { x: number; y: number } | null; others: { x: number; y: number }[]; disabled: boolean }>()
const emit = defineEmits<{ walk: [seat: Seat] }>()
const near = (a: { x: number; y: number }, b: Seat) => Math.hypot(a.x-b.x, a.y-b.y) <= 18
function state(seat: Seat) {
  if (props.self && near(props.self, seat)) return 'kamu di sini'
  return props.others.some((p) => near(p, seat)) ? 'terisi' : 'kosong'
}
</script>

<template>
  <section v-if="seats.length" class="seat-picker" aria-label="Tempat duduk">
    <h3>Tempat duduk</h3>
    <p>Pilih kursi kosong untuk berjalan ke sana.</p>
    <div class="seats">
      <button v-for="(seat, index) in seats" :key="seat.key" type="button"
        :aria-label="`Kursi ${index + 1}: ${state(seat)}`" :aria-pressed="state(seat) === 'kamu di sini'"
        :disabled="disabled || state(seat) !== 'kosong'" @click="emit('walk', seat)">{{ index + 1 }}</button>
    </div>
    <p role="note">Status berdasarkan posisi peserta, bukan reservasi kursi.</p>
  </section>
</template>

<style scoped>
h3 { font: var(--weight-semibold) var(--text-lg)/var(--line-tight) var(--kv-font-display); }
p { color: var(--kv-text-muted); font-size: var(--text-sm); }
.seats { display: grid; grid-template-columns: repeat(auto-fit, minmax(var(--touch-min), 1fr)); gap: var(--space-2); }
button {
  min-width: var(--touch-min); min-height: var(--touch-min);
  border: 1px solid var(--kv-line); border-radius: var(--kv-radius-button);
  background: var(--kv-surface-soft); color: var(--kv-ink); font: inherit; cursor: pointer;
}
button:disabled { opacity: 0.5; cursor: default; }
button[aria-pressed='true'] { background: var(--kv-primary-soft); color: var(--kv-primary-soft-text); opacity: 1; }
button:focus-visible { outline: 2px solid var(--kv-primary); outline-offset: 2px; }
</style>
