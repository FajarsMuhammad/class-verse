<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'

import { errorMessage } from '@/api/client'
import { inviteApi } from '@/api/orgs'
import type { Invite, InviteRole, Role } from '@/api/types'

const props = defineProps<{ orgId: string; myRole: Role; myId: string; classId?: string }>()

const invites = ref<Invite[]>([])
const nextCursor = ref<string | null>(null)
const role = ref<InviteRole>('participant')
const days = ref(7)
const maxUses = ref('')
const error = ref('')
const status = ref('')
const busy = ref(false)
const ids = { role: useId(), days: useId(), uses: useId() }

const canInviteTeachers = computed(() => props.myRole === 'admin')
const roleLabel: Record<InviteRole, string> = { teacher: 'Pengajar', participant: 'Peserta' }

async function load(cursor?: string) {
  try {
    const page = await inviteApi.list(props.orgId, cursor, props.classId)
    invites.value = cursor ? [...invites.value, ...page.items] : page.items
    nextCursor.value = page.next_cursor
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function create() {
  busy.value = true
  error.value = ''
  status.value = ''
  try {
    const uses = maxUses.value.trim() === '' ? null : Number(maxUses.value)
    const inv = await inviteApi.create(props.orgId, {
      role: props.classId ? 'participant' : role.value,
      ...(props.classId ? { class_id: props.classId } : {}),
      expires_in_days: days.value,
      max_uses: uses,
    })
    invites.value = [inv, ...invites.value]
    status.value = `Undangan ${inv.code} dibuat.`
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

async function revoke(inv: Invite) {
  error.value = ''
  status.value = ''
  try {
    await inviteApi.revoke(props.orgId, inv.id)
    invites.value = invites.value.filter((i) => i.id !== inv.id)
    status.value = `Undangan ${inv.code} dicabut.`
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function copy(text: string, what: string) {
  try {
    await navigator.clipboard.writeText(text)
    status.value = `${what} disalin.`
  } catch {
    status.value = `Salin manual: ${text}`
  }
}

function canRevoke(inv: Invite) {
  return props.myRole === 'admin' || inv.created_by === props.myId
}

function expiry(iso: string) {
  return new Date(iso).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
}

onMounted(() => load())
</script>

<template>
  <section class="panel" :class="{ embedded: classId }" aria-labelledby="invite-title">
    <h2 id="invite-title" class="title">{{ classId ? 'Undang siswa ke kelas ini' : 'Undang anggota' }}</h2>
    <p v-if="classId" class="muted">Undangan peserta. Siswa yang sudah punya kelas di tahun ajaran ini tidak dipindahkan; hubungi admin.</p>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>
    <p class="sr-status" role="status">{{ status }}</p>

    <form class="create" @submit.prevent="create">
      <div v-if="!classId" class="field">
        <label :for="ids.role" class="label">Sebagai</label>
        <select :id="ids.role" v-model="role" class="input">
          <option value="participant">Peserta</option>
          <option v-if="canInviteTeachers" value="teacher">Pengajar</option>
        </select>
      </div>
      <div class="field">
        <label :for="ids.days" class="label">Berlaku</label>
        <select :id="ids.days" v-model.number="days" class="input">
          <option v-for="d in 7" :key="d" :value="d">{{ d }} hari</option>
        </select>
      </div>
      <div class="field">
        <label :for="ids.uses" class="label">Batas pemakaian</label>
        <input
          :id="ids.uses"
          v-model="maxUses"
          class="input"
          inputmode="numeric"
          placeholder="Tanpa batas"
        />
      </div>
      <button type="submit" class="btn-primary" :disabled="busy">
        {{ busy ? 'Membuat…' : 'Buat undangan' }}
      </button>
    </form>

    <p v-if="(classId ? invites.filter(i => i.class_id === classId) : invites).length === 0" class="empty">Belum ada undangan aktif.</p>
    <ul v-else class="list">
      <li v-for="inv in (classId ? invites.filter(i => i.class_id === classId) : invites)" :key="inv.id" class="item">
        <div class="meta">
          <span class="code">{{ inv.code }}</span>
          <span class="badge">{{ roleLabel[inv.role] }}</span>
          <span class="muted">
            Berlaku sampai {{ expiry(inv.expires_at) }} · dipakai {{ inv.used_count
            }}{{ inv.max_uses ? ` dari ${inv.max_uses}` : '' }}
          </span>
        </div>
        <div class="actions">
          <button type="button" class="btn-secondary" @click="copy(inv.code, 'Kode')">
            Salin kode
          </button>
          <button type="button" class="btn-secondary" @click="copy(inv.link, 'Link')">
            Salin link
          </button>
          <button
            v-if="canRevoke(inv)"
            type="button"
            class="btn-secondary"
            :aria-label="`Cabut undangan ${inv.code}`"
            @click="revoke(inv)"
          >
            Cabut
          </button>
        </div>
      </li>
    </ul>
    <button v-if="nextCursor" type="button" class="btn-secondary" @click="load(nextCursor)">
      Muat lagi
    </button>
  </section>
</template>

<style scoped>
.panel.embedded { padding: var(--space-3) 0 0; border: 0; border-top: 1px solid var(--kv-line); border-radius: 0; box-shadow: none; }
.embedded .title { font-size: var(--text-md); }
.embedded .item { background: transparent; border-radius: 0; border-bottom: 1px solid var(--kv-line); padding: var(--space-3) 0; }
.panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding: var(--space-6) var(--space-8);
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
}

.title {
  margin: 0;
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  font-size: var(--text-2xl);
}

.sr-status:empty {
  display: none;
}

.sr-status {
  margin: 0;
  font-size: var(--text-md);
  color: var(--color-live-ink);
}

.create {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: var(--space-4);
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.label {
  font-weight: var(--weight-bold);
  font-size: var(--text-md);
}

.input {
  min-height: var(--touch-min);
  padding: 0 var(--space-3);
  border: 1px solid var(--color-input-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-ink);
  font: inherit;
}

.input:focus-visible {
  outline: 2px solid var(--color-primary);
  border-color: var(--color-primary);
}

.empty,
.muted {
  color: var(--color-ink-muted);
  font-size: var(--text-md);
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.item {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-4);
  border-radius: var(--radius-lg);
  background: var(--color-bg);
}

.meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
}

.code {
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  font-size: var(--text-xl);
  letter-spacing: 0.05em;
}

.badge {
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-pill);
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
  font-size: var(--text-sm);
  font-weight: var(--weight-bold);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
</style>
