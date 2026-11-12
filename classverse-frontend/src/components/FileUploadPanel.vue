<script setup lang="ts">
import { onBeforeUnmount, ref, useId } from 'vue'
import { errorMessage } from '@/api/client'
import { uploadFile } from '@/api/uploads'

const props = defineProps<{ orgId: string; userId: string }>()
const inputID = useId()
const selected = ref<File | null>(null)
const busy = ref(false)
const sent = ref(0)
const message = ref('')
const error = ref('')
let controller: AbortController | null = null

function select(event: Event) {
  selected.value = (event.target as HTMLInputElement).files?.[0] ?? null
  sent.value = 0
  message.value = ''
  error.value = ''
}
async function upload() {
  const file = selected.value
  if (!file || busy.value) return
  if (file.size < 1 || file.size > 50_000_000) { error.value = 'Ukuran file harus 1 byte–50 MB'; return }
  const kind = file.type === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf') ? 'pdf' : 'audio'
  busy.value = true
  error.value = ''
  controller = new AbortController()
  try {
    await uploadFile(props.orgId, props.userId, file, kind, (n) => { sent.value = n }, controller.signal)
    message.value = 'Upload tersimpan privat. File belum diterbitkan; validasi isi belum tersedia.'
  } catch (err) {
    if (controller.signal.aborted) message.value = 'Upload dijeda. Tekan Unggah / lanjutkan untuk meneruskan chunk tersimpan.'
    else error.value = `${errorMessage(err)} Pilih file yang sama lalu lanjutkan; chunk tersimpan tidak dikirim ulang.`
  } finally {
    busy.value = false
  }
}
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <section class="upload-panel" :aria-labelledby="`${inputID}-title`">
    <h2 :id="`${inputID}-title`">Unggah file privat</h2>
    <p>Maksimal 50 MB. Chunk tersimpan bisa dilanjutkan selama 24 jam. Setelah reload, pilih file yang sama.</p>
    <p>Fitur tahap uji: upload belum menerbitkan materi atau buku.</p>
    <label :for="inputID">PDF atau audio</label>
    <input :id="inputID" type="file" accept="application/pdf,audio/*" :disabled="busy" @change="select" />
    <progress v-if="selected" :value="sent" :max="selected.size" aria-label="Progres upload" />
    <div class="actions">
      <button type="button" :disabled="!selected || busy" @click="upload">Unggah / lanjutkan</button>
      <button v-if="busy" type="button" @click="controller?.abort()">Jeda</button>
    </div>
    <p v-if="message" role="status">{{ message }}</p>
    <p v-if="error" role="alert">{{ error }}</p>
  </section>
</template>

<style scoped>
.upload-panel { margin-top: var(--space-6); padding: var(--space-5); border: 1px solid var(--color-border); border-radius: var(--radius-lg); }
.actions { display: flex; flex-wrap: wrap; gap: var(--space-3); margin-top: var(--space-3); }
button, input { min-height: 44px; }
progress { display: block; width: 100%; margin-top: var(--space-3); }
</style>
