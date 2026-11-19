<script setup lang="ts">
import PagedSelect from './PagedSelect.vue'
import RemotePagination from './RemotePagination.vue'
import { computed, onMounted, ref, useId, watch } from 'vue'
import StatusTabs from './StatusTabs.vue'
import { ApiError, errorMessage } from '@/api/client'
import { fileApi } from '@/api/files'
import { hasMore } from '@/api/page'
import { formatLabel, libraryApi, shelves } from '@/api/library'
import type { Book, BookInput } from '@/api/types'
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
const pageNumber = ref(1)
const cursors = ref<(string | undefined)[]>([undefined])
async function movePage(delta: number) {
 if (delta > 0) cursors.value = [...cursors.value.slice(0, pageNumber.value), next.value ?? undefined]
 pageNumber.value += delta
 await loadPage()
}
async function loadPage() { await load(true) }
const next = ref<string | null>(null)
const statusFilter = ref('all')
const formatFilter = ref<BookInput['format'] | ''>('')
const levelFilter = ref<BookInput['level'] | ''>('')
const search = ref('')
const counts = ref({ all: 0, published: 0, draft: 0 })
const statusTabs = computed(() => [
  { key: 'all', label: 'Semua', count: counts.value.all },
  { key: 'published', label: 'Terbit', count: counts.value.published },
  { key: 'draft', label: 'Draf', count: counts.value.draft },
])
let searchTimer: ReturnType<typeof setTimeout> | undefined
watch([statusFilter, formatFilter, levelFilter], () => void load())
watch(search, () => { clearTimeout(searchTimer); searchTimer = setTimeout(() => void load(), 250) })
const editing = ref<string | null>(null)
const form = ref<BookInput>(blank())
const fields = ref<Record<string, string>>({})
const busy = ref(false)
const message = ref('')
const error = ref('')
const reading = ref<ReadableFile | null>(null)

// Only files matching the format can be chosen; the server checks this again.

async function load(more = false) {
  if (!more) { pageNumber.value = 1; cursors.value = [undefined] }
  try {
    const page = await libraryApi.list(props.orgId, {
      drafts: true, cursor: more ? cursors.value[pageNumber.value - 1] : null, counts: true,
      status: statusFilter.value === 'published' || statusFilter.value === 'draft' ? statusFilter.value : '', format: formatFilter.value, level: levelFilter.value, q: search.value,
    })
    if (page.counts) counts.value = page.counts
    books.value = page.items
    next.value = page.next_cursor
  } catch (err) {
    error.value = errorMessage(err)
  }
}
async function fileOptions(offset: number) {
  const result = await fileApi.list(props.orgId, offset, form.value.format === 'audio' ? 'audio' : 'pdf')
  return { items: result.items.map(f => ({ value: f.id, label: f.name })), more: hasMore(offset, result) }
}

function edit(book?: Book) {
  editing.value = book?.id ?? 'new'
  fields.value = {}
  message.value = ''
  form.value = book
    ? { title: book.title, author: book.author, shelf: book.shelf, level: book.level, format: book.format,
        copies: book.copies, file_id: book.file_id ?? null, cover: book.cover, published: book.published }
    : blank()

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

async function publish(book: Book) {
  error.value = ''
  try {
    await libraryApi.update(props.orgId, book.id, { title: book.title, author: book.author, shelf: book.shelf, level: book.level,
      format: book.format, copies: book.copies, file_id: book.file_id ?? null, cover: book.cover, published: true })
    await load()
    message.value = `${book.title} terbit.`
  } catch (err) {
    error.value = errorMessage(err)
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
      <h2 :id="`${id}-title`" class="title">Buku perpustakaan</h2>
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
          <PagedSelect :id="`${id}-file`" :key="form.format" :model-value="form.file_id ?? ''" label="File" placeholder="Pilih file" :load="fileOptions" @update:model-value="form.file_id = $event || null" />
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

    <div class="toolbar">
      <StatusTabs v-model="statusFilter" :items="statusTabs" label="Status buku" :controls="`${id}-books`" />
      <label>Jenis
        <select v-model="formatFilter"><option value="">Semua jenis</option><option v-for="(label, key) in formatLabel" :key="key" :value="key">{{ label }}</option></select>
      </label>
      <label>Jenjang
        <select v-model="levelFilter"><option value="">Semua jenjang</option><option v-for="l in ['SD', 'SMP', 'SMA', 'Umum']" :key="l" :value="l">{{ l }}</option></select>
      </label>
    </div>
    <label :for="`${id}-search`" class="sr">Cari judul atau penulis</label>
    <input :id="`${id}-search`" v-model="search" type="search" class="search" maxlength="100" placeholder="Cari judul atau penulis…" />
    <p v-if="!books.length && (search || formatFilter || levelFilter || statusFilter !== 'all')" class="empty">Tidak ada buku yang cocok.</p>
    <div :id="`${id}-books`" class="list">
      <BookCard v-for="b in books" :key="b.id" :book="b" @open="open">
        <div class="row-actions">
          <button v-if="!b.published" type="button" class="btn-primary" @click="publish(b)">Terbitkan<span class="sr"> {{ b.title }}</span></button>
          <button type="button" class="btn-secondary" @click="edit(b)">Ubah<span class="sr"> {{ b.title }}</span></button>
          <button type="button" class="btn-secondary" @click="archive(b)">Arsipkan<span class="sr"> {{ b.title }}</span></button>
        </div>
      </BookCard>
      <button v-if="!editing" type="button" class="add-card" @click="edit()">
        <strong>+ Tambah buku</strong><span>E-book dari PDF di Berkas, audio, atau buku fisik</span>
      </button>
    </div>
    <RemotePagination v-if="pageNumber > 1 || next" :page="pageNumber" :more="!!next" @move="movePage" />

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
  min-height: var(--touch-min);
  font: inherit;
}
.check {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: var(--touch-min);
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
.toolbar { display: flex; flex-wrap: wrap; align-items: end; gap: var(--space-3); margin-bottom: var(--space-3); }
.toolbar label { display: grid; gap: var(--space-1); font-size: var(--text-sm); }
.toolbar select, .search { min-height: var(--touch-min); padding: var(--space-2); border: 1px solid var(--kv-line); border-radius: var(--radius-md); background: var(--color-surface); color: var(--color-ink); font: inherit; }
.search { width: 100%; box-sizing: border-box; margin-bottom: var(--space-4); }
.add-card { display: grid; place-content: center; gap: var(--space-1); min-height: 180px; padding: var(--space-4); border: 1px dashed var(--kv-primary); border-radius: var(--radius-lg); background: transparent; color: var(--color-ink-muted); font: inherit; text-align: center; cursor: pointer; }
.add-card strong { color: var(--kv-primary); }
.add-card:focus-visible { outline: 2px solid var(--kv-primary); outline-offset: 2px; }
</style>
