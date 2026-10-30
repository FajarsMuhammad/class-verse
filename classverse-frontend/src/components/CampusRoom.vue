<script setup lang="ts">
import { computed, onMounted } from 'vue'

import GameCanvas from '@/components/GameCanvas.vue'
import { useRoom } from '@/realtime/useRoom'

const props = defineProps<{ roomId: string; schoolName: string }>()
const { status, map, world, online, error, sendMove, start } = useRoom()
const statusText = computed(
  () =>
    ({
      connecting: 'Menghubungkan…',
      open: `${online.value} orang online`,
      reconnecting: 'Koneksi terputus. Menghubungkan kembali…',
      replaced: 'Kelasverse dibuka di tab lain. Muat ulang untuk memakai tab ini.',
      denied: 'Tidak dapat masuk peta sekolah. Pilih sekolah kembali.',
      closed: 'Koneksi ditutup.',
    })[status.value],
)

onMounted(() => start(props.roomId, 'campus'))
</script>

<template>
  <section aria-labelledby="map-title">
    <header class="heading">
      <div>
        <h1 id="map-title">{{ schoolName }}</h1>
        <p role="status" data-testid="connection-status">{{ error || statusText }}</p>
      </div>
      <span class="scope">Peta sekolah</span>
    </header>
    <div class="layout">
      <div>
        <div class="map-frame">
          <GameCanvas
            v-if="map"
            :deps="{ map, world, sendMove }"
            label="Peta sekolah. Fokus peta, gunakan WASD atau tombol panah untuk berjalan. Klik tujuan untuk mencari jalan."
          />
          <p v-else class="loading">{{ error || 'Memuat peta sekolah…' }}</p>
          <div v-if="map && status !== 'open'" class="connection-overlay" role="status">
            {{ statusText }}
          </div>
        </div>
        <p class="hint">
          Gerak dengan WASD, tombol panah, atau klik tujuan. Masuk gedung tersedia pada tahap
          berikutnya.
        </p>
      </div>
      <aside class="buildings" aria-labelledby="buildings-title">
        <h2 id="buildings-title">Gedung</h2>
        <ul>
          <li v-for="b in map?.buildings ?? []" :key="b.key" :class="b.key">
            <strong>{{ b.name }}</strong>
            <span>{{ b.staffOnly ? 'Khusus pengajar dan admin' : b.description }}</span>
          </li>
        </ul>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  margin-bottom: var(--space-5);
}
h1 {
  margin: 0;
  font: var(--weight-semibold) var(--text-3xl)/var(--line-tight) var(--font-display);
}
.heading p,
.hint {
  color: var(--color-ink-muted);
  margin: var(--space-2) 0 0;
}
.scope {
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-pill);
  font-weight: var(--weight-bold);
}
.layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 280px;
  gap: var(--space-5);
  align-items: start;
  container-type: inline-size;
}
.map-frame {
  aspect-ratio: 8 / 5;
  width: 100%;
  position: relative;
  background: var(--map-grass);
  border-radius: var(--radius-2xl);
  overflow: hidden;
}
.loading {
  padding: var(--space-8);
}
.connection-overlay {
  position: absolute;
  inset: 0;
  display: grid;
  place-content: center;
  padding: var(--space-8);
  background: var(--color-surface-alt);
  opacity: 0.95;
  text-align: center;
  font-weight: var(--weight-bold);
}
.hint {
  font-size: var(--text-md);
  text-align: center;
  line-height: var(--line-relaxed);
}
.buildings {
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
  padding: var(--space-5);
  min-height: 0;
  box-sizing: border-box;
  max-height: calc((100cqw - 280px - var(--space-5)) * 0.625);
  overflow-y: auto;
}
h2 {
  font: var(--weight-semibold) var(--text-2xl)/var(--line-tight) var(--font-display);
  margin: 0 0 var(--space-4);
}
ul {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: var(--space-3);
}
li {
  padding: var(--space-3);
  border-left: var(--space-1) solid var(--roof-classroom);
  border-radius: var(--radius-sm);
  background: var(--color-bg);
}
li span {
  display: block;
  font-size: var(--text-sm);
  color: var(--color-ink-muted);
  margin-top: var(--space-1);
}
.ruang-guru {
  border-color: var(--roof-staff);
}
.perpustakaan {
  border-color: var(--roof-library);
}
.aula {
  border-color: var(--roof-hall);
}
.kantin {
  border-color: var(--roof-canteen);
}
.taman,
.lapangan {
  border-color: var(--color-speaking);
}
@media (max-width: 900px) {
  .layout {
    grid-template-columns: minmax(0, 1fr);
  }
  .scope {
    display: none;
  }
  .buildings {
    max-height: none;
  }
}
</style>
