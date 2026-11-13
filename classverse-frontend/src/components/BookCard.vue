<script setup lang="ts">
import { computed } from 'vue'
import { dueLabel, formatLabel } from '@/api/library'
import type { Book } from '@/api/types'

// loanable: show Pinjam/Kembalikan (catalog for members; the staff manager leaves it off).
const props = defineProps<{ book: Book; loanable?: boolean; busy?: boolean }>()
defineEmits<{ open: [book: Book]; borrow: [book: Book]; giveBack: [book: Book] }>()

const digital = computed(() => props.book.format !== 'buku')
const action = computed(() => {
  const resume = props.book.position > (props.book.format === 'ebook' ? 1 : 0)
  if (resume) return 'Lanjutkan'
  return props.book.format === 'audio' ? 'Dengarkan' : 'Baca sekarang'
})
const status = computed(() => {
  const b = props.book
  if (digital.value) return 'Selalu tersedia'
  if (b.my_loan) return `Kamu pinjam · kembali ${dueLabel(b.my_loan.due_at)}`
  if (b.copies === 0) return 'Belum ada eksemplar'
  return b.available > 0 ? `${b.available} dari ${b.copies} tersedia` : 'Sedang dipinjam'
})
</script>

<template>
  <article class="book" :aria-label="book.title">
    <div class="cover" :style="{ background: `var(--kv-subject-${book.cover})` }" aria-hidden="true" />
    <div class="body">
      <h3 class="title">{{ book.title }}</h3>
      <p class="author">{{ book.author }}</p>
      <p class="chips">
        <span class="chip">{{ book.level }}</span>
        <span class="chip format">{{ formatLabel[book.format] }}</span>
        <span v-if="book.sample" class="chip">Contoh</span>
        <span v-if="!book.published" class="chip draft">Draf</span>
      </p>
      <div class="foot">
        <span class="status" :class="{ ok: digital || book.available > 0 || book.my_loan }">{{ status }}</span>
        <button v-if="digital && book.file_id" type="button" class="btn-primary act" @click="$emit('open', book)">
          {{ action }}<span class="sr"> {{ book.title }}</span>
        </button>
        <template v-else-if="loanable && !digital">
          <button v-if="book.my_loan" type="button" class="btn-secondary act" :disabled="busy"
            @click="$emit('giveBack', book)">Kembalikan<span class="sr"> {{ book.title }}</span></button>
          <button v-else-if="book.available > 0" type="button" class="btn-primary act" :disabled="busy"
            @click="$emit('borrow', book)">Pinjam<span class="sr"> {{ book.title }}</span></button>
        </template>
      </div>
      <slot />
    </div>
  </article>
</template>

<style scoped>
.book {
  display: flex;
  gap: var(--space-3);
  padding: var(--space-3);
  border: 1px solid var(--color-border-soft);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}
.cover {
  flex: none;
  width: 48px;
  height: 64px;
  border: 2px solid var(--kv-ink);
  border-radius: var(--radius-sm);
}
.body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: var(--space-1);
  min-width: 0;
}
.title,
.author,
.chips {
  margin: 0;
}
.title {
  font-size: var(--text-base);
  overflow-wrap: anywhere;
}
.author {
  color: var(--color-ink-muted);
  font-size: var(--text-sm);
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1);
}
.chip {
  padding: 0 var(--space-2);
  border-radius: var(--radius-pill);
  background: var(--color-surface-alt);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
}
.chip.format {
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
}
.chip.draft {
  background: var(--color-accent-soft);
  color: var(--color-accent-ink);
}
.foot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}
.status {
  color: var(--color-ink-muted);
  font-size: var(--text-sm);
}
.status.ok {
  color: var(--color-live-ink);
}
.act {
  min-height: 44px;
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
</style>
