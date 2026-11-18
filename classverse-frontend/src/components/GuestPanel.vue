<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { guestHostApi, type GuestLink, type GuestRequest } from '@/api/guests'
import { errorMessage } from '@/api/client'

const props = defineProps<{ roomId: string }>()
const links = ref<GuestLink[]>([])
const requests = ref<GuestRequest[]>([])
const hours = ref(2)
const max = ref(40)
const busy = ref(false)
const error = ref('')
const notice = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
let stopped = false
async function load() {
  try {
    const [ls, rs] = await Promise.all([guestHostApi.links(props.roomId), guestHostApi.requests(props.roomId)])
    if (stopped) return
    links.value = ls.items
    requests.value = rs.items
    error.value = ''
  } catch (e) { if (!stopped) error.value = errorMessage(e) }
}
async function poll() { await load(); if (!stopped) timer = setTimeout(poll, 5000) }
async function act(fn: () => Promise<unknown>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try { await fn(); await load() } catch (e) { error.value = errorMessage(e) }
  finally { busy.value = false }
}
function createLink() { return act(() => guestHostApi.create(props.roomId, hours.value, max.value)) }
function revokeLink(id: string) { return act(() => guestHostApi.revoke(props.roomId, id)) }
function decideRequest(id: string, decision: 'approve' | 'deny') { return act(() => guestHostApi.decide(props.roomId, id, decision)) }
async function copy(link: string) {
  try { await navigator.clipboard.writeText(link); notice.value = 'Tautan disalin.' }
  catch { notice.value = 'Pilih tautan lalu salin secara manual.' }
}
onMounted(poll)
onBeforeUnmount(() => { stopped = true; clearTimeout(timer) })
</script>

<template>
  <details class="guest-panel">
    <summary class="btn-secondary">Undang tamu <span v-if="requests.length">({{ requests.length }} menunggu)</span></summary>
    <div class="content">
      <h3>Akses tamu pertemuan</h3>
      <p>Tamu hanya dapat mengikuti ruang ini. Pastikan undangan dan izin orang tua/wali sesuai aturan sekolah.</p>
      <form @submit.prevent="createLink">
        <label>Masa berlaku (jam)<input v-model.number="hours" type="number" min="1" max="24" required /></label>
        <label>Batas tamu<input v-model.number="max" type="number" min="1" max="40" required /></label>
        <button class="btn-primary" :disabled="busy">Buat tautan</button>
      </form>
      <p v-if="error" role="alert">{{ error }}</p>
      <p v-if="notice" role="status">{{ notice }}</p>
      <ul>
        <li v-for="link in links" :key="link.id">
          <label>Tautan tamu<input :value="link.link" readonly @focus="($event.target as HTMLInputElement).select()" /></label>
          <span>Kode {{ link.code }} · {{ link.used_count }}/{{ link.max_guests }} · Berakhir {{ new Date(link.expires_at).toLocaleString('id-ID') }}</span>
          <div class="actions">
            <button class="btn-secondary" @click="copy(link.link)">Salin tautan</button>
            <button class="btn-secondary" :disabled="busy" @click="revokeLink(link.id)">Cabut tautan</button>
          </div>
        </li>
      </ul>
      <h3>Menunggu persetujuan</h3>
      <p v-if="!requests.length">Belum ada permintaan masuk.</p>
      <ul aria-live="polite">
        <li v-for="request in requests" :key="request.id">
          <span>{{ request.display_name }} · Tamu</span>
          <div class="actions">
            <button class="btn-primary" :disabled="busy" :aria-label="`Izinkan ${request.display_name}`" @click="decideRequest(request.id, 'approve')">Izinkan</button>
            <button class="btn-secondary" :disabled="busy" :aria-label="`Tolak ${request.display_name}`" @click="decideRequest(request.id, 'deny')">Tolak</button>
          </div>
        </li>
      </ul>
    </div>
  </details>
</template>

<style scoped>
.guest-panel { position: relative; }
.content { padding: var(--space-4); border: 1px solid var(--kv-line); background: var(--color-surface); border-radius: var(--control-radius); width: min(30rem, 80vw); max-height: 65vh; overflow: auto; }
h3 { font-size: var(--text-base); }
p, span { font-size: var(--text-sm); }
form, label { display: grid; gap: var(--space-2); }
form { gap: var(--space-3); }
input { min-height: var(--touch-min); min-width: 0; width: 100%; box-sizing: border-box; }
ul { list-style: none; padding: 0; }
li { display: grid; gap: var(--space-2); padding-block: var(--space-3); border-bottom: 1px solid var(--kv-line); }
.actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }
</style>
