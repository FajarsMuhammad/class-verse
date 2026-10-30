<script setup lang="ts">
import { ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'

import { ApiError, errorMessage } from '@/api/client'
import { orgApi, slugify } from '@/api/orgs'
import AuthCard from '@/components/AuthCard.vue'
import FormField from '@/components/FormField.vue'
import { useAuthStore } from '@/stores/auth'
import { useOrgStore } from '@/stores/org'

const router = useRouter()
const auth = useAuthStore()
const org = useOrgStore()

const name = ref('')
const slug = ref('')
const slugEdited = ref(false)
const fields = ref<Record<string, string>>({})
const error = ref('')
const busy = ref(false)

watch(name, (n) => {
  if (!slugEdited.value) slug.value = slugify(n)
})

function onSlugInput(v: string) {
  slugEdited.value = true
  slug.value = v
}

async function submit() {
  busy.value = true
  error.value = ''
  fields.value = {}
  try {
    const created = await orgApi.create({ name: name.value.trim(), slug: slug.value.trim() })
    await auth.reload()
    org.select(created.id)
    await router.push({ name: 'manage-rooms' })
  } catch (e) {
    if (e instanceof ApiError) fields.value = e.fields
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AuthCard title="Buat sekolah" subtitle="Kamu akan menjadi admin sekolah ini.">
    <form class="form" novalidate @submit.prevent="submit">
      <p v-if="error" class="alert" role="alert">{{ error }}</p>
      <FormField
        v-model="name"
        label="Nama sekolah atau kampus"
        :maxlength="80"
        hint="3–80 karakter, misalnya SMA Negeri 1 Bandung."
        :error="fields.name"
      />
      <FormField
        :model-value="slug"
        label="Alamat singkat (slug)"
        :maxlength="48"
        hint="Huruf kecil, angka, dan tanda hubung."
        :error="fields.slug"
        @update:model-value="onSlugInput"
      />
      <button type="submit" class="btn-primary" :disabled="busy">
        {{ busy ? 'Membuat…' : 'Buat sekolah' }}
      </button>
      <RouterLink class="btn-secondary" :to="{ name: 'start' }">Kembali</RouterLink>
    </form>
  </AuthCard>
</template>
