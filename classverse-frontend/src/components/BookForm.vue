<script setup lang="ts">
import { computed, onMounted, ref, useId, watch } from 'vue'
import { ApiError, errorMessage } from '@/api/client'
import { fileApi } from '@/api/files'
import { formatLabel, libraryApi, shelves } from '@/api/library'
import { hasMore } from '@/api/page'
import { uploadFile } from '@/api/uploads'
import type { Book, BookInput, FileItem } from '@/api/types'

// "Tambah buku" (docs/design/kelola Kelola-TambahBuku): four numbered steps, live shelf preview, save as draft or publish.
const props = defineProps<{ orgId: string; userId: string; book?: Book }>()
const emit = defineEmits<{ saved: [book: Book]; cancel: [] }>()

// Labels name the actual --kv-subject-* colours.
const covers: { key: BookInput['cover']; label: string }[] = [
  { key: 'ipa', label: 'Hijau tua' }, { key: 'matematika', label: 'Biru' }, { key: 'sejarah', label: 'Cokelat' },
  { key: 'bahasa', label: 'Merah muda' }, { key: 'arena', label: 'Oranye' }, { key: 'diskusi', label: 'Hijau' },
  { key: 'kelas', label: 'Nila' }, { key: 'kreatif', label: 'Ungu' },
]
const kinds: { key: BookInput['format']; note: string }[] = [
  { key: 'buku', note: 'Dipinjam di Perpustakaan. Stok dihitung per eksemplar.' },
  { key: 'ebook', note: 'PDF dibaca langsung di Kelasverse. Selalu tersedia.' },
  { key: 'audio', note: 'MP3 atau M4A didengarkan di Kelasverse.' },
]
const kindTitle: Record<BookInput['format'], string> = { buku: 'Buku fisik', ebook: 'E-book', audio: 'Buku audio' }
const levels: BookInput['level'][] = ['SD', 'SMP', 'SMA', 'Umum']
const MAX_BYTES = 50_000_000

const id = useId()
const b = props.book
const form = ref<BookInput>(b
  ? { title: b.title, author: b.author, shelf: b.shelf, level: b.level, format: b.format, copies: b.copies, file_id: b.file_id ?? null, cover: b.cover, published: b.published }
  : { title: '', author: '', shelf: 'sains', level: 'SD', format: 'ebook', copies: 1, file_id: null, cover: 'ipa', published: false })
const fields = ref<Record<string, string>>({})
const error = ref('')
const busy = ref(false)

const digital = computed(() => form.value.format !== 'buku')
const fileKind = computed(() => (form.value.format === 'audio' ? 'audio' : 'pdf'))
const shelfLabel = computed(() => shelves.find((s) => s.key === form.value.shelf)?.label ?? form.value.shelf)
const coverLabel = computed(() => covers.find((c) => c.key === form.value.cover)?.label ?? '')
const checks = computed(() => [
  { ok: !!form.value.title.trim(), label: form.value.title.trim() ? 'Judul diisi' : 'Judul belum diisi' },
  digital.value
    ? { ok: !!form.value.file_id, label: form.value.file_id ? 'File dipilih' : 'File belum dipilih' }
    : { ok: form.value.copies > 0, label: form.value.copies > 0 ? `${form.value.copies} eksemplar` : 'Belum ada eksemplar' },
  { ok: true, label: `Rak ${shelfLabel.value} · ${form.value.level}` },
])

// Step 3 lists files of the matching kind, paged by the server.
const files = ref<FileItem[]>([])
const filesMore = ref(false)
const filesBusy = ref(false)
async function loadFiles(append = false) {
  if (!digital.value) return
  filesBusy.value = true
  const kind = fileKind.value
  try {
    const offset = append ? files.value.length : 0
    const page = await fileApi.list(props.orgId, offset, kind)
    if (kind !== fileKind.value) return
    files.value = append ? [...files.value, ...page.items] : page.items
    filesMore.value = hasMore(offset, page)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    filesBusy.value = false
  }
}
watch(() => form.value.format, (now, before) => {
  if ((now === 'audio') !== (before === 'audio') || now === 'buku') form.value.file_id = null
  void loadFiles()
})
onMounted(() => loadFiles())

const uploading = ref<File | null>(null)
const sent = ref(0)
async function upload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  const pdf = file.type === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf')
  const audio = /\.(mp3|m4a)$/i.test(file.name)
  if (fileKind.value === 'pdf' ? !pdf : !audio) { error.value = fileKind.value === 'pdf' ? 'Pilih file PDF.' : 'Pilih file MP3 atau M4A.'; return }
  if (file.size < 1 || file.size > MAX_BYTES) { error.value = 'Ukuran file harus 1 byte–50 MB.'; return }
  error.value = ''
  uploading.value = file
  sent.value = 0
  try {
    const done = await uploadFile(props.orgId, props.userId, file, fileKind.value, (n) => (sent.value = n))
    await loadFiles()
    form.value.file_id = done.id // upload id is the file id
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    uploading.value = null
  }
}

async function save(published: boolean) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  fields.value = {}
  const body: BookInput = digital.value ? { ...form.value, copies: 0, published } : { ...form.value, file_id: null, published }
  try {
    const saved = b ? await libraryApi.update(props.orgId, b.id, body) : await libraryApi.create(props.orgId, body)
    emit('saved', saved)
  } catch (err) {
    if (err instanceof ApiError) fields.value = err.fields
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}
const mb = (n: number) => `${(n / 1_000_000).toLocaleString('id-ID', { maximumFractionDigits: 1 })} MB`
</script>

<template>
  <form class="book-form" novalidate :aria-labelledby="`${id}-title`" @submit.prevent="save(true)">
    <header class="intro">
      <h3 :id="`${id}-title`">{{ book ? 'Ubah buku' : 'Tambah buku' }}</h3>
      <p>Isi dari atas ke bawah. Pratinjau di kanan menunjukkan tampilan buku di rak Perpustakaan.</p>
    </header>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>

    <div class="steps">
      <fieldset class="step">
        <legend><span class="num" aria-hidden="true">1</span>Jenis buku</legend>
        <p class="hint">Menentukan cara siswa membaca dan isian berikutnya.</p>
        <div class="kinds">
          <label v-for="k in kinds" :key="k.key" class="kind">
            <input v-model="form.format" class="sr" type="radio" :name="`${id}-format`" :value="k.key" />
            <strong>{{ kindTitle[k.key] }}</strong><span>{{ k.note }}</span>
          </label>
        </div>
        <p v-if="fields.format" class="field-error">{{ fields.format }}</p>
      </fieldset>

      <fieldset class="step">
        <legend><span class="num" aria-hidden="true">2</span>Info buku</legend>
        <p class="hint">Judul wajib diisi. Yang lain bisa diubah kapan saja.</p>
        <label :for="`${id}-t`" class="label">Judul <span class="req" aria-hidden="true">*</span></label>
        <input :id="`${id}-t`" v-model="form.title" class="input" required maxlength="200" placeholder="Contoh: Petualangan di Tata Surya"
          :aria-invalid="!!fields.title" :aria-describedby="fields.title ? `${id}-te` : undefined" />
        <p v-if="fields.title" :id="`${id}-te`" class="field-error">{{ fields.title }}</p>
        <label :for="`${id}-a`" class="label">Penulis <span class="muted">(opsional)</span></label>
        <input :id="`${id}-a`" v-model="form.author" class="input" maxlength="120" placeholder="Nama penulis atau penerbit"
          :aria-invalid="!!fields.author" :aria-describedby="fields.author ? `${id}-ae` : undefined" />
        <p v-if="fields.author" :id="`${id}-ae`" class="field-error">{{ fields.author }}</p>
        <fieldset class="chips">
          <legend class="label">Rak</legend>
          <p class="hint">Siswa menemukan buku di rak ini saat masuk Perpustakaan.</p>
          <label v-for="s in shelves" :key="s.key" class="chip"><input v-model="form.shelf" class="sr" type="radio" :name="`${id}-shelf`" :value="s.key" />{{ s.label }}</label>
        </fieldset>
        <fieldset class="chips">
          <legend class="label">Jenjang</legend>
          <label v-for="l in levels" :key="l" class="chip"><input v-model="form.level" class="sr" type="radio" :name="`${id}-level`" :value="l" />{{ l }}</label>
        </fieldset>
      </fieldset>

      <fieldset v-if="digital" class="step" :aria-busy="filesBusy">
        <legend><span class="num" aria-hidden="true">3</span>File {{ form.format === 'audio' ? 'audio' : 'e-book' }}</legend>
        <p class="hint">Pilih {{ form.format === 'audio' ? 'MP3 atau M4A' : 'PDF' }} dari Berkas.</p>
        <p v-if="!files.length && !filesBusy" class="muted">Belum ada file {{ form.format === 'audio' ? 'audio' : 'PDF' }} di Berkas.</p>
        <label v-for="f in files" :key="f.id" class="file">
          <span class="badge" aria-hidden="true">{{ f.kind === 'pdf' ? 'PDF' : 'Audio' }}</span>
          <span class="file-name"><strong>{{ f.name }}</strong><small>{{ f.kind === 'pdf' ? 'PDF' : 'Audio' }} · {{ mb(f.size) }} · di Berkas</small></span>
          <input v-model="form.file_id" type="radio" :name="`${id}-file`" :value="f.id" />
        </label>
        <button v-if="filesMore" type="button" class="btn-secondary" :disabled="filesBusy" @click="loadFiles(true)">Muat file lain</button>
        <p v-if="fields.file_id" class="field-error">{{ fields.file_id }}</p>
        <div class="drop">
          <p><strong>Belum ada di Berkas?</strong> Upload {{ form.format === 'audio' ? 'MP3/M4A' : 'PDF' }} baru, maksimal 50 MB. File otomatis tersimpan di Berkas.</p>
          <label class="btn-secondary upload" :class="{ disabled: !!uploading }">
            Upload file
            <input class="sr" type="file" :accept="form.format === 'audio' ? '.mp3,.m4a,audio/mpeg,audio/mp4' : '.pdf,application/pdf'" :disabled="!!uploading" @change="upload" />
          </label>
          <progress v-if="uploading" :value="sent" :max="uploading.size" aria-label="Progres upload" />
        </div>
      </fieldset>
      <fieldset v-else class="step">
        <legend><span class="num" aria-hidden="true">3</span>Stok buku</legend>
        <label :for="`${id}-c`" class="label">Eksemplar</label>
        <input :id="`${id}-c`" v-model.number="form.copies" class="input short" type="number" min="0" max="999" inputmode="numeric" :aria-invalid="!!fields.copies" />
        <p v-if="fields.copies" class="field-error">{{ fields.copies }}</p>
      </fieldset>

      <fieldset class="step">
        <legend><span class="num" aria-hidden="true">4</span>Warna sampul</legend>
        <p class="hint">Dipakai sebagai sampul di rak. Pilih warna yang mudah dibedakan dari buku lain.</p>
        <div class="swatches">
          <label v-for="c in covers" :key="c.key" class="swatch" :style="{ background: `var(--kv-subject-${c.key})` }">
            <input v-model="form.cover" class="sr" type="radio" :name="`${id}-cover`" :value="c.key" :aria-label="c.label" />
          </label>
        </div>
        <p class="muted">Dipilih: <strong>{{ coverLabel }}</strong></p>
      </fieldset>
    </div>

    <aside class="side" aria-label="Pratinjau dan simpan">
      <section class="panel">
        <h4>Pratinjau di rak <span class="state" :class="{ live: book?.published }">{{ book?.published ? 'Terbit' : 'Draf' }}</span></h4>
        <div class="shelf" aria-hidden="true">
          <div class="cover" :style="{ background: `var(--kv-subject-${form.cover})` }">
            <strong>{{ form.title.trim() || 'Judul buku' }}</strong><small>{{ form.author.trim() || 'Penulis' }}</small>
          </div>
        </div>
        <p class="p-title">{{ form.title.trim() || 'Judul buku' }}</p>
        <p class="muted">{{ form.author.trim() || 'Penulis' }}</p>
        <p class="tags"><span>{{ form.level }}</span><span>{{ formatLabel[form.format] }}</span><span>{{ shelfLabel }}</span></p>
        <p class="ok">{{ digital ? 'Selalu tersedia' : `${form.copies} eksemplar` }}</p>
      </section>
      <section class="panel">
        <h4>Siap disimpan?</h4>
        <ul class="checks">
          <li v-for="c in checks" :key="c.label" :class="{ ok: c.ok }"><span class="dot" aria-hidden="true" /><span class="sr">{{ c.ok ? 'Selesai:' : 'Belum:' }}</span> {{ c.label }}</li>
        </ul>
        <button type="submit" class="btn-primary wide" :disabled="busy || !!uploading">Simpan &amp; terbitkan</button>
        <button type="button" class="btn-secondary wide" :disabled="busy || !!uploading" @click="save(false)">Simpan sebagai draf</button>
        <button type="button" class="link" :disabled="busy" @click="emit('cancel')">Batal</button>
        <p class="muted small">Draf hanya terlihat pengajar. Buku terbit langsung muncul di rak Perpustakaan.</p>
      </section>
    </aside>
  </form>
</template>

<style scoped>
.book-form { display: grid; grid-template-columns: minmax(0, 1fr) 320px; gap: var(--space-4) var(--space-5); align-items: start; }
.intro, .alert { grid-column: 1 / -1; }
.intro h3 { margin: 0; font: var(--weight-bold) var(--text-2xl)/var(--line-tight, 1.2) var(--font-display); }
.intro p, .hint, .muted { margin: 0; color: var(--color-ink-muted); font-size: var(--text-sm); }
.steps { display: grid; gap: var(--space-4); min-width: 0; }
.step { display: grid; gap: var(--space-2); margin: 0; padding: var(--space-5); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); background: var(--color-surface); min-width: 0; }
.step > legend { float: left; display: flex; align-items: center; gap: var(--space-3); padding: 0; font: var(--weight-bold) var(--text-lg)/1.3 var(--font-display); }
.step > legend + * { clear: both; }
.num { display: inline-grid; place-items: center; width: 28px; height: 28px; border-radius: 50%; background: var(--kv-primary); color: var(--color-surface); font-size: var(--text-sm); }
.kinds { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: var(--space-3); margin-top: var(--space-2); }
.kind { display: grid; gap: var(--space-1); padding: var(--space-4); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); cursor: pointer; }
.kind span { color: var(--color-ink-muted); font-size: var(--text-sm); }
.kind:has(:checked) { border: 2px solid var(--kv-primary); background: var(--kv-primary-soft); }
.label { margin-top: var(--space-2); font-weight: var(--weight-bold); font-size: var(--text-sm); }
.req, .field-error { color: var(--color-accent-ink); }
.field-error { margin: 0; font-size: var(--text-sm); }
.input { min-height: var(--touch-min); padding: var(--space-2) var(--space-3); border: 1px solid var(--kv-line); border-radius: var(--radius-md); font: inherit; background: var(--color-surface); color: var(--color-ink); }
.input.short { max-width: 140px; }
.chips { display: flex; flex-wrap: wrap; gap: var(--space-2); margin: 0; padding: 0; border: 0; }
.chips legend, .chips .hint { flex-basis: 100%; }
.chips legend { float: left; width: 100%; }
.chip { display: inline-flex; align-items: center; min-height: var(--touch-min); padding: 0 var(--space-4); border: 1px solid var(--kv-line); border-radius: var(--radius-pill); font-weight: var(--weight-semibold); cursor: pointer; }
.chip:has(:checked) { border: 2px solid var(--kv-primary); background: var(--kv-primary-soft); color: var(--color-primary-strong); }
.kind:has(:focus-visible), .chip:has(:focus-visible), .swatch:has(:focus-visible), .upload:has(:focus-visible) { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
.file { display: flex; align-items: center; gap: var(--space-3); min-height: var(--touch-min); padding: var(--space-3); border: 1px solid var(--kv-line); border-radius: var(--radius-md); cursor: pointer; }
.file:has(:checked) { border-color: var(--kv-primary); background: var(--kv-primary-soft); }
.file input { width: 22px; height: 22px; margin-left: auto; }
.file-name { display: grid; min-width: 0; }
.file-name strong { overflow-wrap: anywhere; }
.file-name small { color: var(--color-ink-muted); }
.badge { padding: var(--space-2); border-radius: var(--radius-md); background: var(--color-accent-soft); color: var(--color-accent-ink); font-size: var(--text-xs); font-weight: var(--weight-bold); }
.drop { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-3); padding: var(--space-4); border: 1px dashed var(--kv-primary); border-radius: var(--radius-md); }
.drop p { flex: 1 1 240px; margin: 0; font-size: var(--text-sm); }
.drop progress { flex-basis: 100%; }
.upload { position: relative; cursor: pointer; }
.upload.disabled { opacity: 0.5; cursor: progress; }
.swatches { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.swatch { width: var(--touch-min); height: var(--touch-min); border-radius: 50%; cursor: pointer; }
.swatch:has(:checked) { box-shadow: 0 0 0 3px var(--color-surface), 0 0 0 5px var(--color-ink); }
.side { display: grid; gap: var(--space-4); position: sticky; top: var(--space-4); }
.panel { display: grid; gap: var(--space-2); padding: var(--space-5); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); background: var(--color-surface); }
.panel h4 { display: flex; justify-content: space-between; align-items: center; margin: 0; font-size: var(--text-md); }
.state { padding: 0 var(--space-2); border-radius: var(--radius-pill); background: var(--color-accent-soft); color: var(--color-accent-ink); font-size: var(--text-xs); }
.state.live { background: var(--kv-primary-soft); color: var(--color-primary-strong); }
.shelf { display: grid; place-items: center; padding: var(--space-4); border-radius: var(--radius-md); background: var(--color-surface-alt); }
.cover { display: grid; align-content: space-between; width: 140px; aspect-ratio: 3 / 4; padding: var(--space-3); border-radius: var(--radius-md); color: var(--color-surface); }
.cover strong { overflow-wrap: anywhere; }
.p-title { margin: 0; font-weight: var(--weight-bold); font-size: var(--text-lg); overflow-wrap: anywhere; }
.tags { display: flex; flex-wrap: wrap; gap: var(--space-1); margin: 0; }
.tags span { padding: 0 var(--space-2); border-radius: var(--radius-pill); background: var(--kv-primary-soft); color: var(--color-primary-strong); font-size: var(--text-xs); font-weight: var(--weight-bold); }
.ok { margin: 0; color: var(--color-live-ink); font-weight: var(--weight-semibold); font-size: var(--text-sm); }
.checks { display: grid; gap: var(--space-2); margin: 0 0 var(--space-2); padding: 0; list-style: none; }
.checks li { display: flex; align-items: center; gap: var(--space-2); color: var(--color-ink-muted); }
.checks li.ok { color: var(--color-ink); font-weight: var(--weight-semibold); }
.dot { width: 18px; height: 18px; border: 2px solid var(--kv-line); border-radius: 50%; }
.checks li.ok .dot { border-color: var(--color-live-ink); background: var(--color-live-ink); }
.wide { width: 100%; justify-content: center; }
.link { min-height: var(--touch-min); border: 0; background: transparent; color: var(--color-ink-muted); font: inherit; font-weight: var(--weight-bold); cursor: pointer; }
.small { font-size: var(--text-xs); }
@media (max-width: 1000px) {
  .book-form { grid-template-columns: minmax(0, 1fr); }
  .side { position: static; }
}
</style>
