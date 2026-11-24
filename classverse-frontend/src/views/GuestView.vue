<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ApiError, errorMessage } from '@/api/client'
import { guestClient, type GuestPreview, type GuestSession } from '@/api/guests'
import GuestRoom from '@/components/GuestRoom.vue'

const route = useRoute()
const client = guestClient()
const preview = ref<GuestPreview | null>(null)
const session = ref<GuestSession | null>(null)
const name = ref('')
const busy = ref(false)
const error = ref('')
const ended = ref(false)
const requestKey = crypto.randomUUID()
let stopped = false
let timer: ReturnType<typeof setTimeout> | undefined
const code = String(route.params.code)
const message = computed(() => ended.value ? error.value : session.value?.status === 'denied' ? 'Permintaan masuk ditolak pengajar.' : 'Menunggu persetujuan pengajar. Isi ruang belum dapat dilihat.')
function fail(e: unknown) {
  error.value = errorMessage(e)
  if (e instanceof ApiError && [401, 403, 404, 410].includes(e.status)) {
    ended.value = true
    client.clear()
  }
}
async function poll() {
  try { const value = await client.status(); if (!stopped) { session.value = value; error.value = '' } }
  catch (e) { if (!stopped) fail(e) }
  if (!stopped && !ended.value && session.value?.status !== 'denied') timer = setTimeout(poll, 5000)
}
async function join() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    session.value = await client.request(code, name.value, requestKey)
    timer = setTimeout(poll, 5000)
  } catch (e) { fail(e) }
  finally { busy.value = false }
}
function leave() {
  ended.value = true
  error.value = 'Kamu sudah meninggalkan pertemuan.'
  clearTimeout(timer)
  client.clear()
}
onMounted(async () => {
  try { preview.value = await client.preview(code) } catch (e) { fail(e) }
})
watch(() => route.params.code, () => { leave(); error.value = 'Tautan berubah. Muat ulang halaman untuk membuka pertemuan.' })
onBeforeUnmount(() => { stopped = true; clearTimeout(timer); client.clear() })
</script>

<template>
  <GuestRoom v-if="session?.status === 'approved' && !ended" :session="session" :client="client" @leave="leave" />
  <main v-else class="guest-entry">
    <section aria-labelledby="guest-title">
      <p>Kelasverse · Tamu</p>
      <h1 id="guest-title">{{ preview?.room_name ?? 'Pertemuan tamu' }}</h1>
      <p v-if="preview">{{ preview.org_name }}</p>
      <p v-if="session || ended" role="status">{{ message }}</p>
      <form v-else-if="preview" @submit.prevent="join">
        <label for="guest-name">Nama tampilan</label>
        <input id="guest-name" v-model="name" autocomplete="name" minlength="3" maxlength="24" required :disabled="busy" />
        <p>Usia tidak perlu diisi. Pastikan undangan sesuai aturan sekolah dan persetujuan orang tua/wali bila diperlukan.</p>
        <button class="btn-primary" :disabled="busy">{{ busy ? 'Mengirim…' : 'Minta masuk' }}</button>
      </form>
      <p v-if="error && !ended" role="alert">{{ error }}</p>
      <p v-if="!preview && !error" role="status">Memuat pertemuan…</p>
      <p v-if="session?.status === 'pending'">Tetap buka halaman ini. Nama kamu akan terlihat oleh pengajar.</p>
    </section>
  </main>
</template>

<style scoped>
.guest-entry { min-height: 100dvh; display: grid; place-items: center; padding: var(--space-5); box-sizing: border-box; background: var(--color-bg); }
section { width: min(100%, 30rem); }
h1 { font: var(--weight-bold) var(--text-2xl)/var(--line-tight) var(--font-display); }
form { display: grid; gap: var(--space-3); }
input { min-height: var(--touch-min); border: 1px solid var(--kv-line); border-radius: var(--control-radius); padding-inline: var(--space-3); font: inherit; }
p { color: var(--kv-text-muted); line-height: var(--line-normal); }
</style>
