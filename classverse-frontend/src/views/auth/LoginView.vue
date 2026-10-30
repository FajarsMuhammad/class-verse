<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'

import { ApiError, errorMessage } from '@/api/client'
import AuthCard from '@/components/AuthCard.vue'
import FormField from '@/components/FormField.vue'
import { useAuthStore } from '@/stores/auth'
import { usePendingStore } from '@/stores/pending'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const pending = usePendingStore()

const email = ref(pending.email)
const password = ref('')
const error = ref(googleError(route.query.error))
const busy = ref(false)

function googleError(code: unknown): string {
  switch (code) {
    case undefined:
      return ''
    case 'google_cancelled':
      return 'Login Google dibatalkan.'
    case 'google_email_unverified':
      return 'Email akun Google kamu belum terverifikasi.'
    case 'account_conflict':
      return 'Email ini sudah tertaut ke akun Google lain.'
    case 'google_disabled':
      return 'Login Google belum diaktifkan di server ini.'
    default:
      return 'Login Google gagal. Coba lagi.'
  }
}

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await auth.login(email.value, password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.push(auth.needsProfile ? { name: 'complete-profile' } : redirect)
  } catch (e) {
    if (e instanceof ApiError && e.code === 'email_not_verified') {
      pending.email = email.value.trim().toLowerCase()
      await router.push({ name: 'verify' })
      return
    }
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AuthCard title="Masuk" subtitle="Selamat datang kembali di Kelasverse.">
    <form class="form" novalidate @submit.prevent="submit">
      <p v-if="error" class="alert" role="alert">{{ error }}</p>
      <FormField v-model="email" label="Email" type="email" autocomplete="email" inputmode="email" />
      <FormField v-model="password" label="Password" type="password" autocomplete="current-password" />
      <button type="submit" class="btn-primary" :disabled="busy">
        {{ busy ? 'Masuk…' : 'Masuk' }}
      </button>
      <p class="divider">atau</p>
      <a class="btn-secondary" href="/api/auth/google/start">Masuk dengan Google</a>
    </form>
    <p class="footnote">
      Belum punya akun? <RouterLink class="link" :to="{ name: 'register' }">Daftar</RouterLink>
    </p>
  </AuthCard>
</template>
