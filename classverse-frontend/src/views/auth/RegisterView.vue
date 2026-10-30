<script setup lang="ts">
import { reactive, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'

import { authApi } from '@/api/auth'
import { ApiError, errorMessage } from '@/api/client'
import AuthCard from '@/components/AuthCard.vue'
import FormField from '@/components/FormField.vue'
import { usePendingStore } from '@/stores/pending'

const router = useRouter()
const pending = usePendingStore()

const form = reactive({ email: '', password: '', display_name: '', birth_date: '' })
const fields = ref<Record<string, string>>({})
const error = ref('')
const busy = ref(false)

async function submit() {
  busy.value = true
  error.value = ''
  fields.value = {}
  try {
    await authApi.register({ ...form })
    pending.email = form.email.trim().toLowerCase()
    await router.push({ name: 'verify' })
  } catch (e) {
    if (e instanceof ApiError) fields.value = e.fields
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AuthCard title="Daftar" subtitle="Buat akun untuk masuk ke ruang belajar sekolahmu.">
    <form class="form" novalidate @submit.prevent="submit">
      <p v-if="error" class="alert" role="alert">{{ error }}</p>
      <FormField
        v-model="form.display_name"
        label="Nama tampilan"
        autocomplete="nickname"
        :maxlength="24"
        hint="3–24 karakter, terlihat oleh teman sekelas."
        :error="fields.display_name"
      />
      <FormField
        v-model="form.email"
        label="Email"
        type="email"
        autocomplete="email"
        inputmode="email"
        :error="fields.email"
      />
      <FormField
        v-model="form.password"
        label="Password"
        type="password"
        autocomplete="new-password"
        hint="Minimal 10 karakter."
        :error="fields.password"
      />
      <FormField
        v-model="form.birth_date"
        label="Tanggal lahir"
        type="date"
        autocomplete="bday"
        :error="fields.birth_date"
      />
      <button type="submit" class="btn-primary" :disabled="busy">
        {{ busy ? 'Mendaftarkan…' : 'Daftar' }}
      </button>
    </form>
    <p class="footnote">
      Sudah punya akun? <RouterLink class="link" :to="{ name: 'login' }">Masuk</RouterLink>
    </p>
  </AuthCard>
</template>
