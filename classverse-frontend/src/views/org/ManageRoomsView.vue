<script setup lang="ts">
import AppShell from '@/components/AppShell.vue'
import InvitePanel from '@/components/InvitePanel.vue'
import RoomPanel from '@/components/RoomPanel.vue'
import FileUploadPanel from '@/components/FileUploadPanel.vue'
import { useAuthStore } from '@/stores/auth'
import { useOrgStore } from '@/stores/org'

const auth = useAuthStore()
const org = useOrgStore()
</script>

<template>
  <AppShell>
    <header class="head">
      <h1 class="title">Kelola ruang</h1>
      <p class="sub">Atur ruang di setiap gedung. Peserta masuk lewat pintu di peta sekolah.</p>
    </header>
    <RoomPanel v-if="org.active" :key="org.active.id" :org-id="org.active.id" />
    <FileUploadPanel
      v-if="org.active && auth.me && org.active.role !== 'participant'"
      :key="`upload-${org.active.id}-${auth.me.id}`"
      :org-id="org.active.id"
      :user-id="auth.me.id"
    />
    <InvitePanel
      v-if="org.active && auth.me"
      :key="org.active.id"
      :org-id="org.active.id"
      :my-role="org.active.role"
      :my-id="auth.me.id"
    />
  </AppShell>
</template>

<style scoped>
.head {
  margin-bottom: var(--space-6);
}

.title {
  margin: 0;
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  font-size: var(--text-3xl);
  line-height: var(--line-tight);
}

.sub {
  margin: var(--space-2) 0 0;
  color: var(--color-ink-muted);
  font-size: var(--text-lg);
}
</style>
