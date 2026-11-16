<script setup lang="ts">
import SeatPicker from '@/components/SeatPicker.vue'
import ResumeLearning from '@/components/ResumeLearning.vue'
import AssignmentPanel from '@/components/AssignmentPanel.vue'
import { avatarURL } from '@/game/avatars'
import type { Seat } from '@/realtime/mapdata'

// Left sidebar (docs/tasks/mvp2-revamp.md): profile and the current place.
// Persisted module progress is independent of the avatar's room.
defineProps<{
  orgId?: string
  progressKey?: number
  name: string
  avatar: string
  teacher: boolean
  campus: boolean
  roomName?: string
  zoneName: string
  seats: Seat[]
  self: { x: number; y: number } | null
  others: { x: number; y: number }[]
  seatsDisabled: boolean
}>()
const closed = defineModel<boolean>('closed', { required: true })
const emit = defineEmits<{ walk: [seat: Seat] }>()
</script>

<template>
  <aside class="learning-panel" aria-label="Panel belajar">
    <header class="panel-heading">
      <h2 v-if="!closed">Panel belajar</h2>
      <button class="btn-secondary" :aria-label="closed ? 'Buka panel belajar' : 'Ciutkan panel belajar'"
        :aria-expanded="!closed" @click="closed = !closed">{{ closed ? '›' : '‹' }}</button>
    </header>
    <template v-if="!closed">
      <div class="learner">
        <img :src="avatarURL(avatar)" alt="" width="42" height="57" />
        <div><strong>{{ name }}</strong><p>{{ teacher ? 'Pengajar' : 'Peserta' }}</p></div>
      </div>
      <AssignmentPanel v-if="orgId" :key="orgId" :org-id="orgId" />
    <ResumeLearning v-if="orgId" :org-id="orgId" :refresh-key="progressKey" />
      <h3>{{ campus ? 'Jelajahi kampus' : 'Ruang belajar' }}</h3>
      <p class="panel-note">{{ campus ? 'Klik gedung di peta atau buka Daftar gedung, lalu berjalan ke pintunya.' : roomName }}</p>
      <template v-if="!campus">
        <p class="panel-note">{{ zoneName || 'Area bersama' }}</p>
        <SeatPicker :seats="seats" :self="self" :others="others" :disabled="seatsDisabled"
          @walk="(seat) => emit('walk', seat)" />
      </template>
    </template>
  </aside>
</template>

<style scoped>
.learning-panel {
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-4);
  background: var(--kv-surface);
  border-radius: var(--kv-radius-card);
}
.panel-heading { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); }
.panel-heading h2 { margin: 0; font-size: var(--text-lg); }
.panel-heading button { flex: none; padding: 0; min-width: var(--touch-min); }
.panel-note { color: var(--kv-text-muted); font-size: var(--text-sm); }
.learner { display: flex; align-items: center; gap: var(--space-3); margin-top: var(--space-5); }
.learner p { color: var(--kv-text-muted); margin: var(--space-1) 0; }
</style>
