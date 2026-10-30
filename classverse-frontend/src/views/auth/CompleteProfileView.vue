<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { ApiError, errorMessage } from '@/api/client'
import AuthCard from '@/components/AuthCard.vue'
import FormField from '@/components/FormField.vue'
import { useAuthStore } from '@/stores/auth'
import { usePendingStore } from '@/stores/pending'

const router = useRouter()
const auth = useAuthStore()
const pending = usePendingStore()

const birthDate = ref(auth.me?.birth_date ?? '')
const fields = ref<Record<string, string>>({})
const error = ref('')
const busy = ref(false)

async function submit() {
  busy.value = true
  error.value = ''
  fields.value = {}
  try {
    await auth.updateProfile({ birth_date: birthDate.value })
    await router.push(pending.takeRedirect() ?? '/')
  } catch (e) {
    if (e instanceof ApiError) fields.value = e.fields
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AuthCard
    title="Lengkapi profil"
    subtitle="Tanggal lahir dibutuhkan sebelum kamu bisa bergabung ke sekolah."
  >
    <form class="form" novalidate @submit.prevent="submit">
      <p v-if="error" class="alert" role="alert">{{ error }}</p>
      <FormField
        v-model="birthDate"
        label="Tanggal lahir"
        type="date"
        autocomplete="bday"
        :error="fields.birth_date"
      />
      <button type="submit" class="btn-primary" :disabled="busy">
        {{ busy ? 'Menyimpan…' : 'Simpan' }}
      </button>
    </form>
  </AuthCard>
</template>
