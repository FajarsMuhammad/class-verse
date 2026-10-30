<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { authApi } from '@/api/auth'
import { errorMessage } from '@/api/client'
import AuthCard from '@/components/AuthCard.vue'
import FormField from '@/components/FormField.vue'
import { useAuthStore } from '@/stores/auth'
import { usePendingStore } from '@/stores/pending'

const router = useRouter()
const auth = useAuthStore()
const pending = usePendingStore()

const email = ref(pending.email)
const code = ref('')
const error = ref('')
const notice = ref('')
const busy = ref(false)

async function submit() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await auth.verifyEmail(email.value, code.value.trim())
    await router.push(auth.needsProfile ? { name: 'complete-profile' } : { name: 'home' })
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function resend() {
  error.value = ''
  notice.value = ''
  try {
    await authApi.resendOtp(email.value)
    notice.value = 'Jika email terdaftar dan belum diverifikasi, kode baru sudah dikirim. Tunggu 1 menit sebelum meminta lagi.'
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <AuthCard title="Verifikasi email" subtitle="Masukkan 6 digit kode yang kami kirim ke emailmu.">
    <form class="form" novalidate @submit.prevent="submit">
      <p v-if="error" class="alert" role="alert">{{ error }}</p>
      <p v-if="notice" class="notice" role="status">{{ notice }}</p>
      <FormField v-model="email" label="Email" type="email" autocomplete="email" inputmode="email" />
      <FormField
        v-model="code"
        label="Kode verifikasi"
        autocomplete="one-time-code"
        inputmode="numeric"
        :maxlength="6"
      />
      <button type="submit" class="btn-primary" :disabled="busy">
        {{ busy ? 'Memeriksa…' : 'Verifikasi' }}
      </button>
      <button type="button" class="btn-secondary" @click="resend">Kirim ulang kode</button>
    </form>
  </AuthCard>
</template>
