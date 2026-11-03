<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AuthCard from '@/components/AuthCard.vue'
import { useAuthStore } from '@/stores/auth'
import { usePendingStore } from '@/stores/pending'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const pending = usePendingStore()

onMounted(async () => {
  const error = route.query.error
  if (typeof error === 'string') {
    await router.replace({ name: 'login', query: { error } })
    return
  }
  let ok = false
  try {
    ok = await auth.completeGoogleLogin()
  } catch {
    ok = false
  }
  if (!ok) {
    await router.replace({ name: 'login', query: { error: 'google_failed' } })
    return
  }
  await router.replace(
    auth.needsProfile ? { name: 'complete-profile' } : (pending.takeRedirect() ?? '/'),
  )
})
</script>

<template>
  <AuthCard title="Menyelesaikan login…">
    <p role="status">Tunggu sebentar.</p>
  </AuthCard>
</template>
