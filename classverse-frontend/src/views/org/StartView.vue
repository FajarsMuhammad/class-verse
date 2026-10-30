<script setup lang="ts">
import { RouterLink } from 'vue-router'

import AuthCard from '@/components/AuthCard.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
</script>

<template>
  <AuthCard
    :title="`Halo, ${auth.me?.display_name ?? ''}`"
    subtitle="Kamu belum tergabung di sekolah mana pun."
  >
    <div class="form">
      <RouterLink class="btn-primary" :to="{ name: 'join' }">Punya kode undangan</RouterLink>
      <RouterLink class="btn-secondary" :to="{ name: 'create-org' }">Buat sekolah baru</RouterLink>
      <p class="hint">Membuat sekolah hanya untuk guru atau admin berusia 18 tahun ke atas.</p>
    </div>
  </AuthCard>
</template>

<style scoped>
.hint {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-ink-muted);
}
</style>
