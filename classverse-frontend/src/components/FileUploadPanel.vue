<script setup lang="ts">
import ListPagination from './ListPagination.vue'
import { onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'
import { PAGE, lastPage } from '@/api/page'
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
const total = ref(0)
const kind = ref<'' | 'pdf' | 'audio'>('')
const dragging = ref(false)
const kindTabs = [{ key: '', label: 'Semua' }, { key: 'pdf', label: 'PDF' }, { key: 'audio', label: 'Audio' }] as const
watch(pageNumber, () => void load())
watch(kind, () => { if (pageNumber.value === 1) void load(); else pageNumber.value = 1 })
function drop(event: DragEvent) {
  dragging.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file && !busy.value) { selected.value = file; sent.value = 0; message.value = ''; error.value = '' }
}
const used = ref(0)
const quota = ref(0)
const reading = ref<FileItem | null>(null)
let controller: AbortController | null = null

const MAX_BYTES = 50_000_000
const mb = (n: number) => `${(n / 1_000_000).toLocaleString('id-ID', { maximumFractionDigits: 1 })} MB`

async function load() {
  try {
    const list = await fileApi.list(props.orgId, (pageNumber.value - 1) * PAGE, kind.value)
    // Deleting the last file of the last page steps back instead of showing an empty page.
    const last = lastPage(pageNumber.value, list)
    if (last !== pageNumber.value) { pageNumber.value = last; return }
    files.value = list.items
    total.value = list.total
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
    <h2 :id="`${inputID}-title`" class="title">Berkas</h2>
    <p class="sub">File belajar untuk modul dan perpustakaan. PDF, MP3, atau M4A, maksimal 50 MB per file.</p>
    <div v-if="quota" class="usage">
      <strong>Penyimpanan sekolah</strong><span>{{ mb(used) }} dari {{ mb(quota) }}</span>
      <progress :value="used" :max="quota" aria-label="Penyimpanan sekolah terpakai" />
    </div>

    <div class="drop" :class="{ dragging }" @dragover.prevent="dragging = true" @dragleave="dragging = false" @drop.prevent="drop">
      <div>
        <strong>Tarik file ke sini atau pilih dari komputer</strong>
        <p>Upload yang terputus bisa dilanjutkan sampai 24 jam; setelah reload, pilih file yang sama.</p>
        <p v-if="selected" class="picked">{{ selected.name }} · {{ mb(selected.size) }}</p>
      </div>
      <label :for="inputID" class="btn-primary pick">Pilih file</label>
      <input :id="inputID" class="sr" type="file" accept="application/pdf,.pdf,audio/mpeg,.mp3,audio/mp4,.m4a" :disabled="busy" @change="select" />
    </div>
    <progress v-if="selected" :value="sent" :max="selected.size" aria-label="Progres upload" />
    <div v-if="selected" class="actions">
      <button type="button" class="btn-primary" :disabled="busy" @click="upload">Unggah / lanjutkan</button>
      <button v-if="busy" type="button" class="btn-secondary" @click="controller?.abort()">Jeda</button>
    </div>
    <p v-if="message" class="notice" role="status">{{ message }}</p>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>

    <div class="tabs" role="group" aria-label="Jenis berkas">
      <button v-for="t in kindTabs" :key="t.key" type="button" class="btn-secondary" :aria-pressed="kind === t.key" @click="kind = t.key">
        {{ t.label }}<span v-if="kind === t.key"> {{ total }}</span>
      </button>
    </div>
    <p v-if="!files.length" class="empty">{{ kind ? 'Belum ada file jenis ini.' : 'Belum ada file.' }}</p>
    <div v-else class="table-wrap">
      <table class="list" aria-label="Daftar berkas">
        <thead><tr><th scope="col">File</th><th scope="col">Jenis</th><th scope="col">Ukuran</th><th scope="col">Dipakai di</th><th scope="col"><span class="sr">Aksi</span></th></tr></thead>
        <tbody>
          <tr v-for="f in files" :key="f.id" class="item">
            <th scope="row" class="name">{{ f.name }}</th>
            <td>{{ f.kind === 'pdf' ? 'PDF' : 'Audio' }}</td>
            <td>{{ mb(f.size) }}</td>
            <td><small>{{ f.used_by?.length ? f.used_by.join(', ') : 'Belum dipakai' }}</small></td>
            <td class="row-actions">
              <button type="button" class="btn-secondary" @click="reading = f">
                {{ f.position > (f.kind === 'pdf' ? 1 : 0) ? 'Lanjutkan' : f.kind === 'pdf' ? 'Baca' : 'Putar' }}
                <span class="sr">{{ f.name }}</span>
              </button>
              <button v-if="isAdmin || f.uploaded_by === userId" type="button" class="btn-secondary" :disabled="!!f.used_by?.length"
                :title="f.used_by?.length ? 'File masih dipakai; arsipkan dulu pemakaiannya.' : undefined" @click="remove(f)">
                Hapus <span class="sr">{{ f.name }}</span>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="hint">File yang masih dipakai di modul, buku, atau tugas tidak bisa dihapus; arsipkan dulu pemakaiannya.</p>
    <ListPagination v-model="pageNumber" :total="total" :size="PAGE" label="Halaman berkas" />
    <FileReader v-if="reading" :key="reading.id" :org-id="orgId" :file="reading" @close="closed" />
  </section>
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
progress {
  width: 100%;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
.usage { display: grid; grid-template-columns: 1fr auto; gap: var(--space-2); padding: var(--space-4); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); }
.usage progress { grid-column: 1 / -1; }
.drop { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-4); padding: var(--space-5); border: 1px dashed var(--kv-primary); border-radius: var(--radius-lg); }
.drop.dragging { background: var(--kv-primary-soft); }
.drop p { margin: var(--space-1) 0 0; color: var(--color-ink-muted); }
.drop .picked { color: var(--color-ink); font-weight: var(--weight-semibold); }
.pick { display: inline-flex; align-items: center; min-height: var(--touch-min); cursor: pointer; }
.drop:focus-within .pick { outline: 2px solid var(--kv-primary); outline-offset: 2px; }
.tabs { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.table-wrap { overflow-x: auto; }
table.list { width: 100%; border-collapse: collapse; }
table.list th, table.list td { padding: var(--space-3); border-bottom: 1px solid var(--kv-line); text-align: left; vertical-align: middle; }
.row-actions { display: flex; flex-wrap: wrap; gap: var(--space-2); justify-content: flex-end; }
.hint { margin: 0; color: var(--color-ink-muted); font-size: var(--text-sm); }
.name { overflow-wrap: anywhere; }
</style>
