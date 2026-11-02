<script setup lang="ts">
import { ref, watch } from 'vue'

import { errorMessage } from '@/api/client'
import { reportApi } from '@/api/reports'
import type { ModerationAction, Report, ReportStatus } from '@/api/types'
import AppShell from '@/components/AppShell.vue'
import { useOrgStore } from '@/stores/org'

const org = useOrgStore()
const filter = ref<ReportStatus>('open')
const reports = ref<Report[]>([])
const cursor = ref<string | null>(null)
const loading = ref(false)
const error = ref('')
const busy = ref('')

const actions = ref<ModerationAction[]>([])
const actionCursor = ref<string | null>(null)
const actionError = ref('')

const filters: { value: ReportStatus; label: string }[] = [
  { value: 'open', label: 'Baru' },
  { value: 'reviewed', label: 'Ditinjau' },
  { value: 'dismissed', label: 'Diabaikan' },
]
const statusLabel: Record<ReportStatus, string> = {
  open: 'Baru',
  reviewed: 'Ditinjau',
  dismissed: 'Diabaikan',
}
const actionLabel: Record<ModerationAction['action'], string> = {
  mute: 'membisukan',
  unmute: 'mengaktifkan chat',
  kick: 'mengeluarkan',
  allow: 'mengizinkan masuk lagi',
  lock: 'mengunci ruang',
  unlock: 'membuka kunci ruang',
}

let generation = 0
async function load(next?: string) {
  if (!org.active) return
  const id = ++generation
  loading.value = true
  error.value = ''
  try {
    const page = await reportApi.list(org.active.id, { status: filter.value, cursor: next })
    if (id !== generation) return
    reports.value = next ? [...reports.value, ...page.items] : page.items
    cursor.value = page.next_cursor
  } catch (e) {
    if (id === generation) error.value = errorMessage(e)
  } finally {
    if (id === generation) loading.value = false
  }
}

async function loadLog(next?: string) {
  if (!org.active || !org.isAdmin) return
  actionError.value = ''
  try {
    const page = await reportApi.moderationLog(org.active.id, next)
    actions.value = next ? [...actions.value, ...page.items] : page.items
    actionCursor.value = page.next_cursor
  } catch (e) {
    actionError.value = errorMessage(e)
  }
}

async function review(r: Report, status: 'reviewed' | 'dismissed') {
  busy.value = r.id
  error.value = ''
  try {
    await reportApi.review(r.id, status)
    reports.value = reports.value.filter((x) => x.id !== r.id)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = ''
  }
}

watch(
  () => [org.active?.id, filter.value],
  () => {
    reports.value = []
    cursor.value = null
    void load()
  },
  { immediate: true },
)
watch(
  () => org.active?.id,
  () => {
    actions.value = []
    void loadLog()
  },
  { immediate: true },
)

const when = (iso: string) =>
  new Date(iso).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
</script>

<template>
  <AppShell>
    <header class="head">
      <h1 class="title">Moderasi</h1>
      <p class="sub">
        Laporan dari peserta di sekolah ini. Isi chat dihapus otomatis setelah 30 hari.
      </p>
    </header>

    <section class="panel" aria-labelledby="reports-title">
      <h2 id="reports-title">Laporan</h2>
      <div class="filters" role="group" aria-label="Status laporan">
        <button
          v-for="f in filters"
          :key="f.value"
          type="button"
          class="btn-secondary"
          :aria-pressed="filter === f.value"
          @click="filter = f.value"
        >
          {{ f.label }}
        </button>
      </div>
      <p v-if="error" role="alert" class="error">{{ error }}</p>
      <p v-if="loading && !reports.length" role="status">Memuat laporan…</p>
      <p v-else-if="!reports.length" class="empty">
        Tidak ada laporan {{ statusLabel[filter].toLowerCase() }}.
      </p>
      <ul class="list">
        <li v-for="r in reports" :key="r.id" class="item">
          <p class="meta">
            <strong>{{ r.reporter.name }}</strong> melaporkan <strong>{{ r.target.name }}</strong>
            <span v-if="r.room_name"> di {{ r.room_name }}</span> ·
            <time :datetime="r.created_at">{{ when(r.created_at) }}</time>
          </p>
          <p v-if="r.message_text" class="quote">“{{ r.message_text }}”</p>
          <p v-else-if="r.message_id === null && r.room_id" class="muted">
            Laporan pengguna, tanpa pesan.
          </p>
          <p class="reason"><span class="label">Alasan:</span> {{ r.reason }}</p>
          <div v-if="r.status === 'open'" class="actions">
            <button
              type="button"
              class="btn-primary"
              :disabled="busy === r.id"
              @click="review(r, 'reviewed')"
            >
              Tandai ditinjau
            </button>
            <button
              type="button"
              class="btn-secondary"
              :disabled="busy === r.id"
              @click="review(r, 'dismissed')"
            >
              Abaikan
            </button>
          </div>
          <p v-else class="muted">{{ statusLabel[r.status] }}</p>
        </li>
      </ul>
      <button
        v-if="cursor"
        type="button"
        class="btn-secondary"
        :disabled="loading"
        @click="load(cursor)"
      >
        Muat berikutnya
      </button>
    </section>

    <section v-if="org.isAdmin" class="panel" aria-labelledby="log-title">
      <h2 id="log-title">Log moderasi</h2>
      <p v-if="actionError" role="alert" class="error">{{ actionError }}</p>
      <p v-else-if="!actions.length" class="empty">Belum ada aksi moderasi.</p>
      <ul class="list">
        <li v-for="a in actions" :key="a.id" class="log-item">
          <strong>{{ a.actor.name }}</strong> {{ actionLabel[a.action] }}
          <strong v-if="a.target">{{ a.target.name }}</strong> di {{ a.room_name }} ·
          <time :datetime="a.created_at">{{ when(a.created_at) }}</time>
        </li>
      </ul>
      <button
        v-if="actionCursor"
        type="button"
        class="btn-secondary"
        @click="loadLog(actionCursor)"
      >
        Muat berikutnya
      </button>
    </section>
  </AppShell>
</template>

<style scoped>
.head {
  margin-bottom: var(--space-6);
}
.title {
  margin: 0;
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  font-size: var(--text-3xl);
  line-height: var(--line-tight);
}
.sub {
  margin: var(--space-2) 0 0;
  color: var(--color-ink-muted);
  font-size: var(--text-lg);
}
.panel {
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
  padding: var(--space-6);
  margin-bottom: var(--space-6);
}
h2 {
  margin: 0 0 var(--space-4);
  font: var(--weight-semibold) var(--text-2xl)/var(--line-tight) var(--font-display);
}
.filters {
  display: flex;
  gap: var(--space-2);
  margin-bottom: var(--space-4);
}
.filters [aria-pressed='true'] {
  border-color: var(--color-primary);
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
}
.list {
  list-style: none;
  padding: 0;
  margin: 0 0 var(--space-4);
  display: grid;
  gap: var(--space-3);
}
.item {
  padding: var(--space-4);
  border: 2px solid var(--color-border-soft);
  border-radius: var(--radius-lg);
}
.meta,
.reason {
  margin: 0 0 var(--space-2);
}
.quote {
  margin: 0 0 var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-alt);
  border-radius: var(--radius-md);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.reason {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.label {
  font-weight: var(--weight-bold);
}
.actions {
  display: flex;
  gap: var(--space-2);
}
.muted,
.empty,
.meta time,
.log-item time {
  color: var(--color-ink-muted);
}
.error {
  font-weight: var(--weight-bold);
}
.log-item {
  padding: var(--space-2) 0;
  border-bottom: 2px solid var(--color-border-soft);
}
</style>
