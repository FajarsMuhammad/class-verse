<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppShell from '@/components/AppShell.vue'
import InvitePanel from '@/components/InvitePanel.vue'
import RoomPanel from '@/components/RoomPanel.vue'
import FileUploadPanel from '@/components/FileUploadPanel.vue'
import LibraryManager from '@/components/LibraryManager.vue'
import ModuleManager from '@/components/ModuleManager.vue'
import QuizManager from '@/components/QuizManager.vue'
import ClassManager from '@/components/ClassManager.vue'
import AssignmentManager from '@/components/AssignmentManager.vue'
import GradebookReport from '@/components/GradebookReport.vue'
import { useAuthStore } from '@/stores/auth'
import { useOrgStore } from '@/stores/org'

// Full-page fallback for phones (list mode has no map, so no in-room tool rail). One section at a time;
// on desktop the same tools open from Ruang Guru and Perpustakaan.
const auth = useAuthStore()
const org = useOrgStore()
const route = useRoute()
const router = useRouter()
const sections = [
  { key: 'ruang', label: 'Ruang & undangan' },
  { key: 'modul', label: 'Modul & kuis' },
  { key: 'kelas', label: 'Kelas & tugas' },
  { key: 'perpustakaan', label: 'Perpustakaan' },
  { key: 'rapor', label: 'Rapor' },
] as const
type Section = (typeof sections)[number]['key']
const section = computed<Section>(() => sections.find((s) => s.key === route.query.bagian)?.key ?? 'ruang')
const pick = (key: Section) => router.replace({ query: { ...route.query, bagian: key } })
</script>

<template>
  <AppShell>
    <header class="head">
      <h1 class="title">Kelola sekolah</h1>
      <p class="sub">Di peta, alat guru ada di Ruang Guru dan alat perpustakaan ada di Perpustakaan.</p>
    </header>
    <nav class="sections" aria-label="Bagian kelola">
      <button v-for="s in sections" :key="s.key" type="button" class="btn-secondary" :aria-pressed="section === s.key"
        @click="pick(s.key)">{{ s.label }}</button>
    </nav>
    <template v-if="org.active && auth.me">
      <template v-if="section === 'ruang'">
        <RoomPanel :key="org.active.id" :org-id="org.active.id" />
        <InvitePanel :key="`invite-${org.active.id}`" :org-id="org.active.id" :my-role="org.active.role" :my-id="auth.me.id" />
      </template>
      <template v-else-if="section === 'modul'">
        <ModuleManager :key="`modules-${org.active.id}`" :org-id="org.active.id" @assign="router.replace({ query: { ...route.query, bagian: 'kelas', modul: $event } })" />
        <QuizManager :key="`quizzes-${org.active.id}`" :org-id="org.active.id" />
        <FileUploadPanel :key="`upload-${org.active.id}`" :org-id="org.active.id" :user-id="auth.me.id"
          :is-admin="org.active.role === 'admin'" />
      </template>
      <template v-else-if="section === 'kelas'">
        <h2 class="section-title">Kelas</h2>
        <ClassManager :key="`classes-${org.active.id}`" :org-id="org.active.id" :user-id="auth.me?.id" :is-admin="org.active.role === 'admin'" view="classes" />
        <h2 class="section-title">Siswa</h2>
        <ClassManager :key="`students-${org.active.id}`" :org-id="org.active.id" :is-admin="org.active.role === 'admin'" view="students" />
        <template v-if="org.active.role === 'admin'">
          <h2 class="section-title">Tahun ajaran</h2>
          <ClassManager :key="`years-${org.active.id}`" :org-id="org.active.id" is-admin view="years" />
        </template>
        <AssignmentManager :key="`assignments-${org.active.id}`" :org-id="org.active.id" :module-id="typeof route.query.modul === 'string' ? route.query.modul : undefined" />
      </template>
      <GradebookReport v-else-if="section === 'rapor'" embedded />
      <template v-else>
        <FileUploadPanel :key="`library-upload-${org.active.id}`" :org-id="org.active.id" :user-id="auth.me.id"
          :is-admin="org.active.role === 'admin'" />
        <LibraryManager :key="`library-${org.active.id}`" :org-id="org.active.id" :user-id="auth.me.id" />
        <ModuleManager :key="`library-modules-${org.active.id}`" :org-id="org.active.id" building-key="perpus" />
      </template>
    </template>
  </AppShell>
</template>

<style scoped>
.head { margin-bottom: var(--space-4); }
.section-title { margin: var(--space-6) 0 var(--space-3); font: var(--weight-semibold) var(--text-lg)/var(--line-tight) var(--font-display); }
.title { margin: 0; font-family: var(--font-display); font-weight: var(--weight-semibold); font-size: var(--text-3xl); line-height: var(--line-tight); }
.sub { margin: var(--space-2) 0 0; color: var(--color-ink-muted); font-size: var(--text-lg); }
.sections { display: flex; flex-wrap: wrap; gap: var(--space-2); margin-bottom: var(--space-4); }
.sections [aria-pressed='true'] { background: var(--color-primary-soft); color: var(--color-primary-strong); }
</style>
