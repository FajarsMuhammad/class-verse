<script setup lang="ts">
import SeatPicker from '@/components/SeatPicker.vue'
import ResumeLearning from '@/components/ResumeLearning.vue'
import AssignmentPanel from '@/components/AssignmentPanel.vue'
import { avatarURL } from '@/game/avatars'
import type { Seat } from '@/realtime/mapdata'

// "Belajar" item of the side menu (docs/screens/sidemenu.png): profile, tasks, resume and the current place.
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
const emit = defineEmits<{ walk: [seat: Seat] }>()
</script>

<template>
  <div class="learning-panel">
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
  </div>
</template>

<style scoped>
.learning-panel { display: flex; flex-direction: column; gap: var(--space-3); }
.panel-note { margin: 0; color: var(--kv-text-muted); font-size: var(--text-sm); }
h3 { margin: var(--space-2) 0 0; }
.learner { display: flex; align-items: center; gap: var(--space-3); }
.learner p { color: var(--kv-text-muted); margin: var(--space-1) 0; }
</style>
