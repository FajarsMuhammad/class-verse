<script setup lang="ts">
import { onBeforeUnmount, ref, useId, watch } from 'vue'
import { errorMessage } from '@/api/client'
import { dueLabel, libraryApi, shelves } from '@/api/library'
import type { Book, Loan, Shelf } from '@/api/types'
import BookCard from './BookCard.vue'
import FileReader, { type ReadableFile } from './FileReader.vue'

const props = defineProps<{ orgId: string }>()
// v-model:shelf lets the map select a shelf; clicking the active tab clears it.
const shelf = defineModel<Shelf | ''>('shelf', { default: '' })

const ids = { search: useId(), results: useId(), loans: useId() }
const query = ref('')
const books = ref<Book[]>([])
const next = ref<string | null>(null)
const loading = ref(false)
const error = ref('')
const reading = ref<ReadableFile | null>(null)
const loans = ref<Loan[]>([])
const maxLoans = ref(0)
const busy = ref(false)
const status = ref('')
let generation = 0
let timer: ReturnType<typeof setTimeout> | undefined

const SEARCH_DELAY_MS = 300

async function load(more = false) {
  const mine = ++generation
  loading.value = true
  error.value = ''
  try {
    const page = await libraryApi.list(props.orgId, { shelf: shelf.value, q: query.value, cursor: more ? next.value : null })
    // A newer search started while this one was in flight; drop the stale page.
    if (mine !== generation) return
    books.value = more ? [...books.value, ...page.items] : page.items
    next.value = page.next_cursor
  } catch (err) {
    if (mine === generation) error.value = errorMessage(err)
  } finally {
    if (mine === generation) loading.value = false
  }
}

async function loadLoans() {
  try {
    const page = await libraryApi.loans(props.orgId)
    loans.value = page.items
    maxLoans.value = page.max_active
  } catch (err) {
    error.value = errorMessage(err)
  }
}

// After borrow/return the server is the source of truth for stock and Pinjamanku.
async function act(fn: () => Promise<unknown>, done: string) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  status.value = ''
  let failed = ''
  try {
    await fn()
    status.value = done
  } catch (err) {
    failed = errorMessage(err)
  } finally {
    await Promise.all([load(), loadLoans()])
    // Reloading clears list errors; keep the reason the action was refused.
    if (failed) error.value = failed
    busy.value = false
  }
}

const borrow = (b: Book) => act(() => libraryApi.borrow(props.orgId, b.id), `${b.title} dipinjam.`)
const giveBack = (id: string, title: string) => act(() => libraryApi.giveBack(props.orgId, id), `${title} dikembalikan.`)

function open(book: Book) {
  if (!book.file_id) return
  reading.value = { id: book.file_id, name: book.title, kind: book.format === 'audio' ? 'audio' : 'pdf', position: book.position }
}

function closed(position: number) {
  const id = reading.value?.id
  books.value = books.value.map((b) => (b.file_id === id ? { ...b, position } : b))
  reading.value = null
}

watch(shelf, () => load(), { immediate: true })
loadLoans()
watch(query, () => {
  clearTimeout(timer)
  timer = setTimeout(() => load(), SEARCH_DELAY_MS)
})
onBeforeUnmount(() => clearTimeout(timer))

const shelfLabel = (key: string) => shelves.find((s) => s.key === key)?.label ?? key
</script>

<template>
  <section class="catalog" aria-label="Katalog perpustakaan">
    <label :for="ids.search" class="sr">Cari judul atau penulis</label>
    <input :id="ids.search" v-model="query" type="search" class="search" placeholder="Cari judul atau penulis"
      :aria-controls="ids.results" />
    <div class="shelves" role="group" aria-label="Rak">
      <button v-for="s in shelves" :key="s.key" type="button" class="shelf" :aria-pressed="shelf === s.key"
        @click="shelf = shelf === s.key ? '' : s.key">{{ s.label }}</button>
    </div>
    <section v-if="loans.length" class="loans" :aria-labelledby="ids.loans">
      <h3 :id="ids.loans" class="heading">Pinjamanku ({{ loans.length }}/{{ maxLoans }})</h3>
      <ul>
        <li v-for="l in loans" :key="l.id">
          <span><strong>{{ l.title }}</strong><br /><small>Kembali {{ dueLabel(l.due_at) }} WIB</small></span>
          <button type="button" class="btn-secondary act" :disabled="busy" @click="giveBack(l.id, l.title)">
            Kembalikan<span class="sr"> {{ l.title }}</span>
          </button>
        </li>
      </ul>
    </section>
    <p class="sr" role="status">{{ status }}</p>
    <div :id="ids.results" class="results" aria-live="polite">
      <h3 class="heading">{{ shelf ? `Rak ${shelfLabel(shelf)}` : 'Semua rak' }}</h3>
      <p v-if="error" class="alert" role="alert">{{ error }} <button type="button" class="link" @click="load()">Coba lagi</button></p>
      <p v-else-if="!loading && !books.length" class="empty">
        {{ query || shelf ? 'Tidak ada buku yang cocok.' : 'Belum ada buku di katalog.' }}
      </p>
      <BookCard v-for="b in books" :key="b.id" :book="b" loanable :busy="busy" @open="open" @borrow="borrow"
        @give-back="(x) => x.my_loan && giveBack(x.my_loan.id, x.title)" />
      <p v-if="loading" role="status" class="empty">Memuat…</p>
      <button v-else-if="next" type="button" class="btn-secondary" @click="load(true)">Muat lagi</button>
    </div>
    <FileReader v-if="reading" :key="reading.id" :org-id="orgId" :file="reading" @close="closed" />
  </section>
</template>

<style scoped>
.catalog {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  min-height: 0;
}
.search {
  min-height: var(--touch-min);
  padding: 0 var(--space-3);
  border: 1px solid var(--color-input-border);
  border-radius: var(--radius-lg);
  font: inherit;
}
.shelves {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.shelf {
  min-height: var(--touch-min);
  padding: 0 var(--space-3);
  border: none;
  border-radius: var(--radius-pill);
  background: var(--color-surface-alt);
  color: var(--color-ink);
  font: inherit;
  font-weight: var(--weight-semibold);
  cursor: pointer;
}
.shelf[aria-pressed='true'] {
  background: var(--kv-subject-bahasa);
  color: var(--color-surface);
}
.shelf:focus-visible,
.search:focus-visible {
  outline: 3px solid var(--color-primary-strong);
  outline-offset: 2px;
}
.loans ul {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: var(--space-2) 0 0;
  padding: 0;
  list-style: none;
}
.loans li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-lg);
  background: var(--color-surface-alt);
}
.loans small {
  color: var(--color-ink-muted);
}
.act {
  min-height: var(--touch-min);
}
.results {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.heading {
  margin: 0;
  font-size: var(--text-md);
}
.empty {
  margin: 0;
  color: var(--color-ink-muted);
}
</style>
