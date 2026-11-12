<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'
import { ApiError, errorMessage } from '@/api/client'
import { fileApi } from '@/api/files'
import { formatLabel, libraryApi, shelves } from '@/api/library'
import type { Book, BookInput, FileItem } from '@/api/types'
import BookCard from './BookCard.vue'
import FileReader, { type ReadableFile } from './FileReader.vue'

const props = defineProps<{ orgId: string }>()

const covers: { key: BookInput['cover']; label: string }[] = [
  { key: 'ipa', label: 'Hijau tua' }, { key: 'matematika', label: 'Biru' }, { key: 'sejarah', label: 'Cokelat' },
  { key: 'bahasa', label: 'Merah muda' }, { key: 'kreatif', label: 'Oranye' }, { key: 'diskusi', label: 'Hijau' },
  { key: 'kelas', label: 'Ungu' }, { key: 'arena', label: 'Merah' },
]
const blank = (): BookInput => ({
  title: '', author: '', shelf: 'dongeng', level: 'SD', format: 'buku', copies: 1, file_id: null, cover: 'kelas', published: false,
})

const id = useId()
const books = ref<Book[]>([])
const next = ref<string | null>(null)
const files = ref<FileItem[]>([])
const editing = ref<string | null>(null)
const form = ref<BookInput>(blank())
const fields = ref<Record<string, string>>({})
const busy = ref(false)
const message = ref('')
const error = ref('')
const reading = ref<ReadableFile | null>(null)

// Only files matching the format can be chosen; the server checks this again.
const choices = computed(() => files.value.filter((f) => f.kind === (form.value.format === 'audio' ? 'audio' : 'pdf')))

async function load(more = false) {
  try {
    const page = await libraryApi.list(props.orgId, { drafts: true, cursor: more ? next.value : null })
    books.value = more ? [...books.value, ...page.items] : page.items
    next.value = page.next_cursor
  } catch (err) {
    error.value = errorMessage(err)
  }
}
async function loadFiles() {
  try {
    files.value = (await fileApi.list(props.orgId)).items
  } catch {
    files.value = []
  }
}

function edit(book?: Book) {
  editing.value = book?.id ?? 'new'
  fields.value = {}
  message.value = ''
  form.value = book
    ? { title: book.title, author: book.author, shelf: book.shelf, level: book.level, format: book.format,
        copies: book.copies, file_id: book.file_id ?? null, cover: book.cover, published: book.published }
    : blank()
  void loadFiles()
}

async function save() {
  busy.value = true
  error.value = ''
  fields.value = {}
  const body: BookInput = form.value.format === 'buku'
    ? { ...form.value, file_id: null }
    : { ...form.value, copies: 0 }
  try {
    const saved = editing.value === 'new'
      ? await libraryApi.create(props.orgId, body)
      : await libraryApi.update(props.orgId, editing.value!, body)
    editing.value = null
    await load()
    message.value = `${saved.title} ${saved.published ? 'terbit' : 'disimpan sebagai draf'}.`
  } catch (err) {
    if (err instanceof ApiError) fields.value = err.fields
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}

async function archive(book: Book) {
  if (!window.confirm(`Arsipkan ${book.title}? Buku hilang dari katalog; filenya tetap ada.`)) return
  error.value = ''
  try {
    await libraryApi.archive(props.orgId, book.id)
    await load()
    message.value = `${book.title} diarsipkan.`
  } catch (err) {
    error.value = errorMessage(err)
  }
}

// Staff preview drafts in the same reader participants use.
function open(book: Book) {
  if (book.file_id) reading.value = { id: book.file_id, name: book.title, kind: book.format === 'audio' ? 'audio' : 'pdf', position: book.position }
}

onMounted(() => load())
</script>

<template>
  <section class="manager" :aria-labelledby="`${id}-title`">
    <header class="head">
      <h2 :id="`${id}-title`" class="title">Katalog perpustakaan</h2>
      <button v-if="!editing" type="button" class="btn-primary" @click="edit()">+ Tambah buku</button>
    </header>
    <p class="sub">Draf hanya terlihat oleh pengajar. Buku terbit muncul di ruang Perpustakaan.</p>
    <p v-if="message" class="notice" role="status">{{ message }}</p>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>

    <form v-if="editing" class="form" novalidate @submit.prevent="save">
      <h3 class="form-title">{{ editing === 'new' ? 'Buku baru' : 'Ubah buku' }}</h3>
      <div class="grid">
        <label :for="`${id}-t`">Judul</label>
        <input :id="`${id}-t`" v-model="form.title" required maxlength="200" :aria-invalid="!!fields.title"
          :aria-describedby="fields.title ? `${id}-te` : undefined" />
        <p v-if="fields.title" :id="`${id}-te`" class="field-error">{{ fields.title }}</p>

        <label :for="`${id}-a`">Penulis</label>
        <input :id="`${id}-a`" v-model="form.author" required maxlength="120" :aria-invalid="!!fields.author"
          :aria-describedby="fields.author ? `${id}-ae` : undefined" />
        <p v-if="fields.author" :id="`${id}-ae`" class="field-error">{{ fields.author }}</p>

        <label :for="`${id}-s`">Rak</label>
        <select :id="`${id}-s`" v-model="form.shelf">
          <option v-for="s in shelves" :key="s.key" :value="s.key">{{ s.label }}</option>
        </select>

        <label :for="`${id}-l`">Jenjang</label>
        <select :id="`${id}-l`" v-model="form.level">
          <option v-for="l in ['SD', 'SMP', 'SMA', 'Umum']" :key="l" :value="l">{{ l }}</option>
        </select>

        <label :for="`${id}-f`">Format</label>
        <select :id="`${id}-f`" v-model="form.format" @change="form.file_id = null">
          <option v-for="(label, key) in formatLabel" :key="key" :value="key">{{ label }}</option>
        </select>

        <template v-if="form.format === 'buku'">
          <label :for="`${id}-c`">Eksemplar</label>
          <input :id="`${id}-c`" v-model.number="form.copies" type="number" min="0" max="999" inputmode="numeric"
            :aria-invalid="!!fields.copies" />
          <p v-if="fields.copies" class="field-error">{{ fields.copies }}</p>
        </template>
        <template v-else>
          <label :for="`${id}-file`">File</label>
          <select :id="`${id}-file`" v-model="form.file_id" :aria-invalid="!!fields.file_id"
            :aria-describedby="`${id}-fh`">
            <option :value="null" disabled>Pilih file</option>
            <option v-for="f in choices" :key="f.id" :value="f.id">{{ f.name }}</option>
          </select>
          <p :id="`${id}-fh`" :class="fields.file_id ? 'field-error' : 'hint'">
            {{ fields.file_id || (choices.length ? (form.format === 'audio' ? 'MP3 atau M4A dari File belajar.' : 'PDF dari File belajar.') : 'Belum ada file yang cocok. Unggah dulu di File belajar.') }}
          </p>
        </template>

        <label :for="`${id}-cv`">Warna sampul</label>
        <select :id="`${id}-cv`" v-model="form.cover">
          <option v-for="c in covers" :key="c.key" :value="c.key">{{ c.label }}</option>
        </select>
      </div>
      <label class="check"><input v-model="form.published" type="checkbox" /> Terbitkan ke katalog</label>
      <div class="actions">
        <button type="submit" class="btn-primary" :disabled="busy">Simpan</button>
        <button type="button" class="btn-secondary" :disabled="busy" @click="editing = null">Batal</button>
      </div>
    </form>

    <p v-if="!books.length" class="empty">Belum ada buku. Tambahkan buku pertama.</p>
    <div class="list">
      <BookCard v-for="b in books" :key="b.id" :book="b" @open="open">
        <div class="row-actions">
          <button type="button" class="btn-secondary" @click="edit(b)">Ubah<span class="sr"> {{ b.title }}</span></button>
          <button type="button" class="btn-secondary" @click="archive(b)">Arsipkan<span class="sr"> {{ b.title }}</span></button>
        </div>
      </BookCard>
    </div>
    <button v-if="next" type="button" class="btn-secondary" @click="load(true)">Muat lagi</button>
    <FileReader v-if="reading" :key="reading.id" :org-id="orgId" :file="reading" @close="reading = null; load()" />
  </section>
</template>

<style scoped>
.manager {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  margin-top: var(--space-6);
  padding: var(--space-5);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}
.head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
}
.title,
.sub,
.form-title {
  margin: 0;
}
.title {
  font-family: var(--font-display);
  font-size: var(--text-xl);
}
.sub,
.hint,
.empty {
  margin: 0;
  color: var(--color-ink-muted);
}
.form {
  margin: 0;
  padding: var(--space-4);
  border-radius: var(--radius-md);
  background: var(--color-surface-alt);
}
.grid {
  display: grid;
  grid-template-columns: minmax(120px, max-content) minmax(0, 1fr);
  align-items: center;
  gap: var(--space-2) var(--space-3);
}
.grid .field-error,
.grid .hint {
  grid-column: 2;
  margin: 0;
  font-size: var(--text-sm);
}
.field-error {
  color: var(--color-accent-ink);
}
.grid input,
.grid select {
  min-height: 44px;
  font: inherit;
}
.check {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 44px;
}
.check input {
  width: 22px;
  height: 22px;
}
.actions,
.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: var(--space-3);
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
@media (max-width: 560px) {
  .grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .grid .field-error,
  .grid .hint {
    grid-column: 1;
  }
}
</style>
