<script setup lang="ts">
import ModuleManager from './ModuleManager.vue'
import type { RoomHandle } from '@/realtime/useRoom'
import QuizManager from './QuizManager.vue'
import ClassManager from './ClassManager.vue'
import AssignmentManager from './AssignmentManager.vue'
import GradebookReport from './GradebookReport.vue'
import RoomPanel from './RoomPanel.vue'
import InvitePanel from './InvitePanel.vue'
import FileUploadPanel from './FileUploadPanel.vue'
import LibraryManager from './LibraryManager.vue'

// Staff tool bodies for the side menu. Reuses the Kelola managers unchanged; `library-modules` limits the
// module manager to the Perpustakaan building.
defineProps<{ tool: string; orgId: string; userId: string; isAdmin: boolean; room?: RoomHandle; assignModule?: string }>()
const emit = defineEmits<{ pinned: []; assign: [moduleId: string]; control: [] }>()
</script>

<template>
  <ModuleManager v-if="tool === 'modules'" :org-id="orgId" :room="room" @pinned="emit('pinned')" @assign="emit('assign', $event)" />
  <ModuleManager v-else-if="tool === 'library-modules'" :org-id="orgId" building-key="perpus" :room="room" @pinned="emit('pinned')" @assign="emit('assign', $event)" />
  <QuizManager v-else-if="tool === 'quizzes'" :org-id="orgId" />
  <ClassManager v-else-if="tool === 'classes' || tool === 'students' || tool === 'years'" :org-id="orgId" :user-id="userId" :is-admin="isAdmin" :view="tool" />
  <AssignmentManager v-else-if="tool === 'assignments'" :org-id="orgId" :module-id="assignModule" />
  <GradebookReport v-else-if="tool === 'gradebook'" embedded />
  <div v-else-if="tool === 'rooms'" class="rooms-layout">
    <RoomPanel :org-id="orgId" :current-room-id="room?.current.value?.room_id" @control="emit('control')" />
    <aside class="rooms-invite" aria-label="Undang anggota"><InvitePanel :org-id="orgId" :my-id="userId" :my-role="isAdmin ? 'admin' : 'teacher'" /></aside>
  </div>
  <FileUploadPanel v-else-if="tool === 'files'" :org-id="orgId" :user-id="userId" :is-admin="isAdmin" />
  <LibraryManager v-else-if="tool === 'books'" :org-id="orgId" :user-id="userId" />
</template>

<style scoped>
.rooms-layout { display: grid; grid-template-columns: minmax(0, 1fr) 320px; gap: var(--space-5); align-items: start; }
@media (max-width: 1100px) { .rooms-layout { grid-template-columns: minmax(0, 1fr); } }
</style>
