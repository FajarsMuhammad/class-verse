<script setup lang="ts">
import { computed, nextTick, onMounted, ref, useId, watch } from 'vue'
import { ApiError, errorMessage } from '@/api/client'
import { fileApi } from '@/api/files'
import { formatLabel, libraryApi, shelves } from '@/api/library'
import { hasMore } from '@/api/page'
import { uploadFile } from '@/api/uploads'
import type { Book, BookInput, FileItem } from '@/api/types'

// "Tambah buku" (docs/design/kelola Kelola-TambahBuku): four numbered steps, live shelf preview, save as draft or publish.
const props = defineProps<{ orgId: string; userId: string; book?: Book }>()
// saved: book stored (catalog may refresh). done: leave the page. A new book stays on the page with a banner (mockup).
const emit = defineEmits<{ saved: [book: Book]; done: []; cancel: [] }>()

// Labels name the actual --kv-subject-* colours.
const covers: { key: BookInput['cover']; label: string }[] = [
  { key: 'ipa', label: 'Hijau tua' }, { key: 'matematika', label: 'Biru' }, { key: 'sejarah', label: 'Cokelat' },
  { key: 'bahasa', label: 'Merah muda' }, { key: 'arena', label: 'Oranye' }, { key: 'diskusi', label: 'Hijau' },
  { key: 'kelas', label: 'Ungu' }, { key: 'kreatif', label: 'Violet' },
]
const kinds: { key: BookInput['format']; note: string; icon: string }[] = [
  { key: 'buku', note: 'Dipinjam di Perpustakaan. Stok dihitung per eksemplar.', icon: 'M4 5a2 2 0 0 1 2-2h13v16H6a2 2 0 0 0-2 2zM4 19V5M9 7h6' },
  { key: 'ebook', note: 'PDF dibaca langsung di Kelasverse. Selalu tersedia.', icon: 'M7 2h10a2 2 0 0 1 2 2v16a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2zM9 7h6M9 11h6M9 15h3' },
  { key: 'audio', note: 'MP3 atau M4A didengarkan di Kelasverse.', icon: 'M4 14v-2a8 8 0 0 1 16 0v2M5 14h3v7H5zM16 14h3v7h-3z' },
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
const titleOk = computed(() => form.value.title.trim().length > 0)
const fileOk = computed(() => !digital.value || !!form.value.file_id)
const checks = computed(() => [
  { ok: titleOk.value, label: titleOk.value ? 'Judul diisi' : 'Judul belum diisi' },
  digital.value
    ? { ok: fileOk.value, label: fileOk.value ? 'File dipilih' : 'File belum dipilih' }
    : { ok: true, label: `Jumlah eksemplar: ${form.value.copies}` },
  { ok: true, label: `Rak ${shelfLabel.value} · ${form.value.level}` },
])
const COPIES_MAX = 99
// Stepper range follows the design (1–99). The API still accepts 0–999, so an old book keeps its stock until the user steps it.
const stepCopies = (by: number) => { form.value.copies = Math.min(COPIES_MAX, Math.max(1, form.value.copies + by)) }
const availability = computed(() => (digital.value ? 'Selalu tersedia' : `${form.value.copies} dari ${form.value.copies} eksemplar tersedia`))
const submitted = ref(false)
const saved = ref<'publish' | 'draft' | null>(null)
const savedTitle = ref('')
const missing = computed(() => [...(titleOk.value ? [] : ['judul']), ...(fileOk.value ? [] : ['file'])])
const root = ref<HTMLElement | null>(null)
// Any edit after saving starts a new book's worth of changes: the success banner goes away.
watch(form, () => { saved.value = null }, { deep: true })
function reset() {
  form.value = { title: '', author: '', shelf: 'sains', level: 'SD', format: 'ebook', copies: 1, file_id: null, cover: 'ipa', published: false }
  submitted.value = false
  saved.value = null
  void loadFiles()
  root.value?.scrollIntoView?.({ block: 'start' })
}

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
  submitted.value = true
  if (missing.value.length) {
    await nextTick()
    root.value?.querySelector<HTMLElement>(titleOk.value ? '.file input' : `#${id}-t`)?.focus()
    return
  }
  busy.value = true
  error.value = ''
  fields.value = {}
  const body: BookInput = digital.value ? { ...form.value, copies: 0, published } : { ...form.value, file_id: null, published }
  try {
    const book = b ? await libraryApi.update(props.orgId, b.id, body) : await libraryApi.create(props.orgId, body)
    emit('saved', book)
    if (b) { emit('done'); return }
    savedTitle.value = book.title
    saved.value = published ? 'publish' : 'draft'
    await nextTick()
    root.value?.scrollIntoView?.({ behavior: 'smooth', block: 'start' })
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
  <form ref="root" class="book-form" novalidate :aria-labelledby="`${id}-title`" @submit.prevent="save(true)">
    <header class="intro">
      <h3 :id="`${id}-title`">{{ book ? 'Ubah buku' : 'Tambah buku' }}</h3>
      <p>Isi dari atas ke bawah. Pratinjau di kanan menunjukkan tampilan buku di rak Perpustakaan.</p>
    </header>
    <div class="banners">
      <div v-if="saved" class="banner ok" role="status">
        <strong>{{ saved === 'publish' ? 'Buku terbit' : 'Tersimpan sebagai draf' }}</strong>
        <span>“{{ savedTitle }}” {{ saved === 'publish' ? 'sudah muncul di rak ' + shelfLabel + ' Perpustakaan.' : 'hanya terlihat pengajar sampai diterbitkan.' }}</span>
        <button type="button" class="link" @click="emit('done')">Lihat katalog</button>
        <button type="button" class="btn-secondary" @click="reset">Tambah buku lain</button>
      </div>
      <p v-if="submitted && missing.length && !saved" class="banner bad" role="alert">Lengkapi dulu: {{ missing.join(' dan ') }}.</p>
      <p v-if="error" class="banner bad" role="alert">{{ error }}</p>
    </div>

    <div class="steps">
      <fieldset class="step">
        <legend><span class="num" aria-hidden="true">1</span>Jenis buku</legend>
        <p class="hint">Menentukan cara siswa membaca dan isian berikutnya.</p>
        <div class="kinds">
          <label v-for="k in kinds" :key="k.key" class="kind">
            <input v-model="form.format" class="sr" type="radio" :name="`${id}-format`" :value="k.key" />
            <span class="kind-top">
              <svg viewBox="0 0 24 24" width="28" height="28" aria-hidden="true"><path :d="k.icon" /></svg>
              <svg v-if="form.format === k.key" class="tick" viewBox="0 0 24 24" width="24" height="24" aria-hidden="true"><circle cx="12" cy="12" r="12" /><path d="M7 12.5l3.5 3.5 6.5-7" /></svg>
            </span>
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
          :aria-invalid="!!fields.title || (submitted && !titleOk)" :aria-describedby="fields.title || (submitted && !titleOk) ? `${id}-te` : undefined" />
        <p v-if="fields.title || (submitted && !titleOk)" :id="`${id}-te`" class="field-error">{{ fields.title || 'Judul belum diisi.' }}</p>
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
        <div v-if="!files.length && !filesBusy" class="empty">
          <strong>Belum ada file {{ form.format === 'audio' ? 'audio' : 'PDF' }} di Berkas</strong>
          <span>{{ form.format === 'audio' ? 'Buku audio butuh file MP3 atau M4A.' : 'E-book butuh file PDF.' }} Upload sekarang, file juga akan tersimpan di Berkas.</span>
          <label class="btn-primary upload" :class="{ disabled: !!uploading }">
            Upload file {{ form.format === 'audio' ? 'audio' : 'PDF' }}
            <input class="sr" type="file" :accept="form.format === 'audio' ? '.mp3,.m4a,audio/mpeg,audio/mp4' : '.pdf,application/pdf'" :disabled="!!uploading" @change="upload" />
          </label>
        </div>
        <label v-for="f in files" :key="f.id" class="file">
          <span class="badge" aria-hidden="true">{{ f.kind === 'pdf' ? 'PDF' : 'Audio' }}</span>
          <span class="file-name"><strong>{{ f.name }}</strong><small>{{ f.kind === 'pdf' ? 'PDF' : 'Audio' }} · {{ mb(f.size) }} · di Berkas</small></span>
          <input v-model="form.file_id" type="radio" :name="`${id}-file`" :value="f.id" />
        </label>
        <button v-if="filesMore" type="button" class="btn-secondary" :disabled="filesBusy" @click="loadFiles(true)">Muat file lain</button>
        <p v-if="fields.file_id || (submitted && !fileOk)" class="field-error">{{ fields.file_id || 'Pilih file dulu sebelum menyimpan.' }}</p>
        <div v-if="files.length" class="drop">
          <p><strong>Belum ada di Berkas?</strong> Upload {{ form.format === 'audio' ? 'MP3/M4A' : 'PDF' }} baru, maksimal 50 MB. File otomatis tersimpan di Berkas.</p>
          <label class="btn-secondary upload" :class="{ disabled: !!uploading }">
            Upload file
            <input class="sr" type="file" :accept="form.format === 'audio' ? '.mp3,.m4a,audio/mpeg,audio/mp4' : '.pdf,application/pdf'" :disabled="!!uploading" @change="upload" />
          </label>
          <progress v-if="uploading" :value="sent" :max="uploading.size" aria-label="Progres upload" />
        </div>
      </fieldset>
      <fieldset v-else class="step">
        <legend><span class="num" aria-hidden="true">3</span>Jumlah eksemplar</legend>
        <p class="hint">Berapa buku fisik yang ada di rak sekolah.</p>
        <div class="stepper">
          <div class="stepper-box" role="group" aria-label="Jumlah eksemplar">
            <button type="button" class="step-btn" aria-label="Kurangi eksemplar" :disabled="form.copies <= 1" @click="stepCopies(-1)">−</button>
            <output aria-live="polite">{{ form.copies }}</output>
            <button type="button" class="step-btn" aria-label="Tambah eksemplar" :disabled="form.copies >= COPIES_MAX" @click="stepCopies(1)">+</button>
          </div>
          <p class="hint">Siswa bisa meminjam selama masih ada eksemplar tersedia. Stok berkurang saat dipinjam dan kembali saat dikembalikan.</p>
        </div>
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
        <h4>Pratinjau di rak <span class="state" :class="{ live: saved === 'publish' || (book?.published && !saved) }">{{ saved === 'publish' || (book?.published && !saved) ? 'Terbit' : 'Draf' }}</span></h4>
        <div class="shelf" aria-hidden="true">
          <div class="cover" :style="{ background: `var(--kv-subject-${form.cover})` }">
            <strong>{{ form.title.trim() || 'Judul buku' }}</strong><small>{{ form.author.trim() || 'Penulis' }}</small>
          </div>
        </div>
        <p class="p-title">{{ form.title.trim() || 'Judul buku' }}</p>
        <p class="muted">{{ form.author.trim() || 'Penulis' }}</p>
        <p class="tags"><span>{{ form.level }}</span><span>{{ formatLabel[form.format] }}</span><span>{{ shelfLabel }}</span></p>
        <p class="ok">{{ availability }}</p>
      </section>
      <section class="panel">
        <h4>Siap disimpan?</h4>
        <ul class="checks">
          <li v-for="c in checks" :key="c.label" :class="{ ok: c.ok }"><svg class="dot" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="10" /><path v-if="c.ok" d="M7 12.5l3.5 3.5 6.5-7" /></svg><span class="sr">{{ c.ok ? 'Selesai:' : 'Belum:' }}</span> {{ c.label }}</li>
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
.intro, .banners { grid-column: 1 / -1; }
.banners { display: grid; gap: var(--space-3); }
.banners:empty { display: none; }
.banner { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-3); margin: 0; padding: var(--space-3) var(--space-4); border-radius: var(--radius-lg); }
.banner.ok { background: var(--kv-ok-soft); color: var(--kv-ok-ink); }
.banner.ok span { flex: 1 1 260px; color: var(--color-ink); }
.banner.bad { background: var(--kv-danger-soft); color: var(--kv-danger-text); font-weight: var(--weight-bold); }
.banner .link { color: var(--kv-primary); }
.intro h3 { margin: 0; font: var(--weight-bold) var(--text-3xl, var(--text-2xl))/1.2 var(--font-display); }
.intro p, .hint, .muted { margin: 0; color: var(--color-ink-muted); font-size: var(--text-sm); }
.steps { display: grid; gap: var(--space-4); min-width: 0; }
.step { display: grid; gap: var(--space-2); margin: 0; padding: var(--space-5); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); background: var(--color-surface); min-width: 0; }
.step > legend { float: left; display: flex; align-items: center; gap: var(--space-3); padding: 0; font: var(--weight-bold) var(--text-lg)/1.3 var(--font-display); }
.step > legend + * { clear: both; }
.num { display: inline-grid; place-items: center; width: 28px; height: 28px; border-radius: 50%; background: var(--kv-primary); color: var(--color-surface); font-size: var(--text-sm); }
.kinds { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: var(--space-3); margin-top: var(--space-2); }
.kind-top { display: flex; align-items: center; justify-content: space-between; color: var(--color-ink-muted); }
.kind:has(:checked) .kind-top { color: var(--kv-primary); }
.tick circle { fill: var(--kv-primary); }
.tick path { fill: none; stroke: var(--color-surface); stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
.kind svg:not(.tick) { fill: none; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
.kind { display: grid; gap: var(--space-1); padding: var(--space-4); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); cursor: pointer; }
.kind span { color: var(--color-ink-muted); font-size: var(--text-sm); }
.kind:has(:checked) { border: 2px solid var(--kv-primary); background: var(--kv-primary-soft); }
.label { margin-top: var(--space-2); font-weight: var(--weight-bold); font-size: var(--text-sm); }
.req, .field-error { color: var(--color-accent-ink); }
.field-error { margin: 0; font-size: var(--text-sm); }
.input { min-height: var(--touch-min); padding: var(--space-2) var(--space-3); border: 1px solid var(--kv-line); border-radius: var(--radius-md); font: inherit; background: var(--color-surface); color: var(--color-ink); }
.stepper { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-4); }
.stepper .hint { flex: 1 1 240px; }
.stepper-box { display: flex; align-items: center; gap: var(--space-2); }
.stepper-box output { min-width: 56px; text-align: center; font: var(--weight-bold) var(--text-2xl)/1 var(--font-display); }
.step-btn { width: 44px; height: 44px; border: 1px solid var(--kv-line); border-radius: var(--radius-md); background: var(--color-surface); color: var(--color-ink); font: var(--weight-bold) 22px/1 var(--font-body); cursor: pointer; }
.step-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.step-btn:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
.empty { display: grid; justify-items: center; gap: var(--space-2); padding: var(--space-5); border-radius: var(--radius-lg); background: var(--kv-page); text-align: center; }
.empty span { max-width: 380px; color: var(--color-ink-muted); font-size: var(--text-sm); }
.input[aria-invalid='true'] { border: 2px solid var(--kv-danger-text); }
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
.cover { display: grid; align-content: space-between; width: 150px; aspect-ratio: 3 / 4; padding: var(--space-3) var(--space-3) var(--space-3) var(--space-5); border-radius: 6px 12px 12px 6px; color: var(--color-surface); box-shadow: inset 8px 0 0 color-mix(in srgb, var(--color-ink) 18%, transparent); }
.cover strong { overflow-wrap: anywhere; }
.p-title { margin: 0; font-weight: var(--weight-bold); font-size: var(--text-lg); overflow-wrap: anywhere; }
.tags { display: flex; flex-wrap: wrap; gap: var(--space-1); margin: 0; }
.tags span { padding: 0 var(--space-2); border-radius: var(--radius-pill); background: var(--kv-primary-soft); color: var(--color-primary-strong); font-size: var(--text-xs); font-weight: var(--weight-bold); }
.ok { margin: 0; color: var(--color-live-ink); font-weight: var(--weight-semibold); font-size: var(--text-sm); }
.checks { display: grid; gap: var(--space-2); margin: 0 0 var(--space-2); padding: 0; list-style: none; }
.checks li { display: flex; align-items: center; gap: var(--space-2); color: var(--color-ink-muted); }
.checks li.ok { color: var(--color-ink); font-weight: var(--weight-semibold); }
.dot { flex: none; width: 22px; height: 22px; }
.dot circle { fill: var(--color-surface); stroke: var(--kv-line); stroke-width: 2; }
.dot path { fill: none; stroke: var(--color-surface); stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
.checks li.ok .dot circle { fill: var(--color-live-ink); stroke: var(--color-live-ink); }
.wide { width: 100%; justify-content: center; }
.link { min-height: var(--touch-min); border: 0; background: transparent; color: var(--color-ink-muted); font: inherit; font-weight: var(--weight-bold); cursor: pointer; }
.small { font-size: var(--text-xs); }
@media (max-width: 1000px) {
  .book-form { grid-template-columns: minmax(0, 1fr); }
  .side { position: static; }
}
</style>
