<script setup lang="ts">
import { useRouter } from 'vue-router'

import AuthCard from '@/components/AuthCard.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()

async function logout() {
  await auth.logout()
  await router.push({ name: 'login' })
}
</script>

<template>
  <AuthCard :title="`Halo, ${auth.me?.display_name ?? ''}`" subtitle="Kamu sudah masuk.">
    <p class="info" data-testid="home-info">
      Gabung ke sekolah lewat kode undangan akan tersedia di langkah berikutnya.
    </p>
    <button type="button" class="btn-secondary" @click="logout">Keluar</button>
  </AuthCard>
</template>

<style scoped>
.info {
  margin: var(--space-6) 0;
  font-size: var(--text-md);
}
</style>
