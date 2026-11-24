<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'

import { errorMessage } from '@/api/client'
import { inviteApi, normalizeInviteCode } from '@/api/orgs'
import type { InvitePreview } from '@/api/types'
import AuthCard from '@/components/AuthCard.vue'
import FormField from '@/components/FormField.vue'
import { useAuthStore } from '@/stores/auth'
import { useOrgStore } from '@/stores/org'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const org = useOrgStore()

const input = ref(typeof route.params.code === 'string' ? route.params.code : '')
const preview = ref<InvitePreview | null>(null)
const code = ref<string | null>(null)
const error = ref('')
const busy = ref(false)
const result = ref('')

const roleLabel = computed(() => (preview.value?.role === 'teacher' ? 'pengajar' : 'peserta'))

async function check() {
  error.value = ''
  preview.value = null
  const normalized = normalizeInviteCode(input.value)
  if (!normalized) {
    error.value = 'Kode undangan terdiri dari 8 huruf/angka, misalnya ABCD-EFGH.'
    return
  }
  busy.value = true
  try {
    preview.value = await inviteApi.preview(normalized)
    code.value = normalized
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function join() {
  if (!code.value) return
  busy.value = true
  error.value = ''
  try {
    const m = await inviteApi.accept(code.value)
    await auth.reload()
    org.select(m.org_id)
    const notices: Record<string, string> = {
      has_class: 'Kamu sudah bergabung ke sekolah, tetapi sudah punya kelas di tahun ajaran ini. Kelasmu tidak diubah. Hubungi admin untuk pindah kelas.',
      class_unavailable: 'Kamu sudah bergabung ke sekolah, tetapi kelas undangan tidak tersedia atau wali kelas sudah berubah. Hubungi admin untuk penempatan kelas.',
      not_student: 'Kamu sudah menjadi anggota sekolah sebagai pengajar/admin. Peranmu tidak diubah dan undangan kelas ini tidak menempatkanmu sebagai siswa.',
    }
    result.value = notices[m.placement_status ?? ''] ?? ''
    if (!result.value) await router.push({ name: 'home' })
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  if (input.value) void check()
})
</script>

<template>
  <AuthCard title="Gabung ke sekolah" subtitle="Masukkan kode undangan dari guru atau admin.">
    <div class="form">
      <p v-if="error" class="alert" role="alert">{{ error }}</p>
      <template v-if="result">
        <p class="notice" role="status">{{ result }}</p>
        <RouterLink class="btn-primary" :to="{ name: 'home' }">Buka sekolah</RouterLink>
      </template>
      <template v-else-if="!preview">
        <form class="form-inner" novalidate @submit.prevent="check">
          <FormField v-model="input" label="Kode undangan" autocomplete="off" :maxlength="12" />
          <button type="submit" class="btn-primary" :disabled="busy">
            {{ busy ? 'Memeriksa…' : 'Periksa kode' }}
          </button>
        </form>
      </template>
      <template v-else>
        <p class="notice" role="status">
          Kamu diundang ke <strong>{{ preview.org_name }}</strong> sebagai {{ roleLabel }}.
          <span v-if="preview.class_name"> Kelas {{ preview.class_name }}, tahun ajaran {{ preview.year_label }}.</span>
        </p>
        <button type="button" class="btn-primary" :disabled="busy" @click="join">
          {{ busy ? 'Bergabung…' : `Gabung ke ${preview.org_name}` }}
        </button>
        <button type="button" class="btn-secondary" @click="preview = null">Pakai kode lain</button>
      </template>
      <RouterLink v-if="org.orgs.length === 0" class="link back" :to="{ name: 'start' }">
        Kembali
      </RouterLink>
      <RouterLink v-else class="link back" :to="{ name: 'home' }">Kembali</RouterLink>
    </div>
  </AuthCard>
</template>

<style scoped>
.form-inner {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.back {
  align-self: center;
  min-height: var(--touch-min);
  display: inline-flex;
  align-items: center;
}
</style>
