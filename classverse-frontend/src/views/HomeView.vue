<script setup lang="ts">
import { ref, watch } from 'vue'

import { orgApi } from '@/api/orgs'
import type { Organization } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import CampusRoom from '@/components/CampusRoom.vue'
import { useOrgStore } from '@/stores/org'

const org = useOrgStore()
const school = ref<Organization | null>(null)
const error = ref('')

watch(
  () => org.active?.id,
  async (id, _, onCleanup) => {
    let stale = false
    onCleanup(() => {
      stale = true
    })
    school.value = null
    error.value = ''
    if (!id) return
    try {
      const result = await orgApi.get(id)
      if (!stale) school.value = result
    } catch {
      if (!stale) error.value = 'Sekolah gagal dimuat. Muat ulang halaman.'
    }
  },
  { immediate: true },
)
</script>

<template>
  <AppShell immersive>
    <CampusRoom
      v-if="school"
      :key="school.id"
      :room-id="school.campus_room_id"
      :school-name="school.name"
    />
    <p v-else role="status">{{ error || 'Memuat sekolah…' }}</p>
  </AppShell>
</template>
