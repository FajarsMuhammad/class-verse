<script setup lang="ts">
import RemotePagination from './RemotePagination.vue'
import { onBeforeUnmount, onMounted, ref, useId } from 'vue'
import { ApiError, errorMessage } from '@/api/client'
import { fileApi } from '@/api/files'
import { uploadFile } from '@/api/uploads'
import type { FileItem } from '@/api/types'
import FileReader from './FileReader.vue'

const props = defineProps<{ orgId: string; userId: string; isAdmin: boolean }>()
const inputID = useId()
const selected = ref<File | null>(null)
const busy = ref(false)
const sent = ref(0)
const message = ref('')
const error = ref('')
const files = ref<FileItem[]>([])
const pageNumber = ref(1)
function movePage(delta: number) { pageNumber.value += delta; void load() }
const used = ref(0)
const quota = ref(0)
const reading = ref<FileItem | null>(null)
let controller: AbortController | null = null

const MAX_BYTES = 50_000_000
const mb = (n: number) => `${(n / 1_000_000).toLocaleString('id-ID', { maximumFractionDigits: 1 })} MB`

async function load() {
  try {
    const list = await fileApi.list(props.orgId, (pageNumber.value - 1) * 20)
    files.value = list.items
    used.value = list.used_bytes
    quota.value = list.quota_bytes
  } catch (err) {
    error.value = errorMessage(err)
  }
}

function select(event: Event) {
  selected.value = (event.target as HTMLInputElement).files?.[0] ?? null
  sent.value = 0
  message.value = ''
  error.value = ''
}

// Server-side byte checks are authoritative; this only gives a quick, friendly message.
function kindOf(file: File): 'pdf' | 'audio' | null {
  const name = file.name.toLowerCase()
  if (file.type === 'application/pdf' || name.endsWith('.pdf')) return 'pdf'
  if (/\.(mp3|m4a)$/.test(name) || ['audio/mpeg', 'audio/mp4', 'audio/x-m4a'].includes(file.type)) return 'audio'
  return null
}

function uploadError(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.code === 'pdf_encrypted' || err.code === 'quota_exceeded') return err.message
    // 400 on complete means the server discarded the file; resuming would not help.
    if (err.status === 400) return 'File bukan PDF, MP3, atau M4A yang valid. Periksa file lalu unggah lagi.'
  }
  return `${errorMessage(err)} Pilih file yang sama lalu tekan Unggah / lanjutkan; bagian yang sudah terkirim tidak dikirim ulang.`
}

async function upload() {
  const file = selected.value
  if (!file || busy.value) return
  const kind = kindOf(file)
  if (!kind) {
    error.value = 'Pilih file PDF, MP3, atau M4A.'
    return
  }
  if (file.size < 1 || file.size > MAX_BYTES) {
    error.value = 'Ukuran file harus 1 byte–50 MB.'
    return
  }
  busy.value = true
  error.value = ''
  message.value = ''
  controller = new AbortController()
  try {
    await uploadFile(props.orgId, props.userId, file, kind, (n) => (sent.value = n), controller.signal)
    selected.value = null
    await load()
    message.value = `${file.name} siap dibaca.`
  } catch (err) {
    if (controller.signal.aborted) message.value = 'Upload dijeda. Tekan Unggah / lanjutkan untuk meneruskan.'
    else error.value = uploadError(err)
  } finally {
    busy.value = false
  }
}

async function remove(file: FileItem) {
  if (!window.confirm(`Hapus ${file.name}? File tidak bisa dikembalikan.`)) return
  error.value = ''
  try {
    await fileApi.remove(props.orgId, file.id)
    await load()
    message.value = `${file.name} dihapus.`
  } catch (err) {
    error.value = err instanceof ApiError && err.code === 'file_in_use' ? err.message : errorMessage(err)
  }
}

function closed(position: number) {
  const id = reading.value?.id
  files.value = files.value.map((f) => (f.id === id ? { ...f, position } : f))
  reading.value = null
}

onMounted(load)
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <section class="upload-panel" :aria-labelledby="`${inputID}-title`">
    <h2 :id="`${inputID}-title`" class="title">File belajar</h2>
    <p class="sub">
      PDF, MP3, atau M4A, maksimal 50 MB. Upload yang terputus bisa dilanjutkan selama 24 jam; setelah reload, pilih
      file yang sama.
    </p>
    <p v-if="quota" class="usage">Terpakai {{ mb(used) }} dari {{ mb(quota) }}</p>

    <label :for="inputID" class="label">Pilih file</label>
    <input
      :id="inputID"
      type="file"
      accept="application/pdf,.pdf,audio/mpeg,.mp3,audio/mp4,.m4a"
      :disabled="busy"
      @change="select"
    />
    <progress v-if="selected" :value="sent" :max="selected.size" aria-label="Progres upload" />
    <div class="actions">
      <button type="button" class="btn-primary" :disabled="!selected || busy" @click="upload">
        Unggah / lanjutkan
      </button>
      <button v-if="busy" type="button" class="btn-secondary" @click="controller?.abort()">Jeda</button>
    </div>
    <p v-if="message" class="notice" role="status">{{ message }}</p>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>

    <p v-if="!files.length" class="empty">Belum ada file.</p>
    <ul v-else class="list">
      <li v-for="f in files" :key="f.id" class="item">
        <span class="name">
          {{ f.name }}
          <small>{{ f.kind === 'pdf' ? 'PDF' : 'Audio' }} · {{ mb(f.size) }}</small>
        </span>
        <button type="button" class="btn-secondary" @click="reading = f">
          {{ f.position > (f.kind === 'pdf' ? 1 : 0) ? 'Lanjutkan' : f.kind === 'pdf' ? 'Baca' : 'Putar' }}
          <span class="sr">{{ f.name }}</span>
        </button>
        <button
          v-if="isAdmin || f.uploaded_by === userId"
          type="button"
          class="btn-secondary"
          @click="remove(f)"
        >
          Hapus <span class="sr">{{ f.name }}</span>
        </button>
      </li>
    </ul>
    <FileReader v-if="reading" :key="reading.id" :org-id="orgId" :file="reading" @close="closed" />
  </section>
  <RemotePagination :page="pageNumber" :more="files.length === 20" @move="movePage" />
</template>

<style scoped>
.upload-panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  margin-top: var(--space-6);
  padding: var(--space-5);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}
.title,
.sub,
.usage {
  margin: 0;
}
.title {
  font-family: var(--font-display);
  font-size: var(--text-xl);
}
.sub,
.usage,
.empty,
small {
  color: var(--color-ink-muted);
}
.label {
  font-weight: var(--weight-semibold);
}
input {
  min-height: var(--touch-min);
}
progress {
  width: 100%;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
}
.list {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}
.item {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3);
  border: 1px solid var(--color-border-soft);
  border-radius: var(--radius-md);
}
.name {
  display: flex;
  flex: 1 1 200px;
  flex-direction: column;
  overflow-wrap: anywhere;
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
</style>
