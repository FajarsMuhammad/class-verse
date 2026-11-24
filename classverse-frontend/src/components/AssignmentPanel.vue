<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { assignmentApi, type Assignment } from '@/api/assignments'
import { errorMessage } from '@/api/client'
import TaskPopup from './learn/TaskPopup.vue'
import { placeInfo } from './learn/place'

// Sidebar launcher for class tasks: a few active tasks; the work itself opens in TaskPopup over the map.
const props = defineProps<{ orgId: string }>()
const SHOWN = 3
const DUE_SOON_MS = 3 * 24 * 60 * 60 * 1000
const tasks = ref<Assignment[]>([])
const busy = ref(false)
const error = ref('')
// null = closed; 'list' = "Lihat semua"; an assignment = open that task.
const popup = ref<Assignment | 'list' | null>(null)
let generation = 0
onBeforeUnmount(() => { generation++ })

const active = computed(() => tasks.value.filter((a) => !a.cancelled && a.done < a.total))
async function load() {
  const mine = ++generation
  busy.value = true
  error.value = ''
  try {
    const page = await assignmentApi.list(props.orgId)
    if (mine === generation) tasks.value = page.items
  } catch (e) {
    if (mine === generation) error.value = errorMessage(e)
  } finally {
    if (mine === generation) busy.value = false
  }
}
onMounted(load)
function close() {
  popup.value = null
  void load()
}
const dueSoon = (a: Assignment) => !!a.due_at && new Date(a.due_at).getTime() - Date.now() <= DUE_SOON_MS
const shortDue = (a: Assignment) => a.due_at
  ? `Tenggat ${new Date(a.due_at).toLocaleString('id-ID', { timeZone: 'Asia/Jakarta', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}`
  : 'Tanpa tenggat'
// Progress ring: circumference of r=18.
const RING = 2 * Math.PI * 18
</script>

<template>
  <section class="tasks" aria-labelledby="tasks-heading" :aria-busy="busy">
    <header>
      <h3 id="tasks-heading">Tugas kelas</h3>
      <span v-if="active.length" class="count">{{ active.length }} aktif</span>
      <button type="button" class="link see-all" @click="popup = 'list'">Lihat semua</button>
    </header>
    <p v-if="error" class="alert" role="alert">{{ error }} <button type="button" class="link" @click="load">Coba lagi</button></p>
    <p v-else-if="busy && !tasks.length" role="status" class="muted">Memuat tugas…</p>
    <p v-else-if="!active.length" class="muted">{{ tasks.length ? 'Semua tugas sudah selesai.' : 'Belum ada tugas kelas.' }}</p>
    <ul class="task-list">
      <li v-for="a in active.slice(0, SHOWN)" :key="a.id" class="card">
        <svg class="ring" viewBox="0 0 44 44" width="44" height="44" aria-hidden="true">
          <circle cx="22" cy="22" r="18" class="track" />
          <circle cx="22" cy="22" r="18" class="fill" :stroke-dasharray="`${RING * (a.total ? a.done / a.total : 0)} ${RING}`" />
          <text x="22" y="26" text-anchor="middle">{{ a.done }}/{{ a.total }}</text>
        </svg>
        <div class="text">
          <strong>{{ a.title }}</strong>
          <small>{{ placeInfo(a.building_key).subject }} · {{ a.class_name }}</small>
        </div>
        <p class="foot">
          <span class="due" :class="{ soon: dueSoon(a) }">{{ shortDue(a) }}</span>
          <button type="button" :class="a.done ? 'btn-primary' : 'btn-secondary start'" @click="popup = a">
            {{ a.done ? 'Lanjutkan' : 'Mulai' }}<span class="sr"> {{ a.title }}</span>
          </button>
        </p>
      </li>
    </ul>
    <TaskPopup v-if="popup" :org-id="orgId" :open="popup === 'list' ? null : popup" @close="close" />
  </section>
</template>

<style scoped>
.tasks { display: grid; gap: var(--space-3); padding: var(--space-3) 0; border-block: 1px solid var(--kv-line); min-width: 0; }
header { display: flex; align-items: center; gap: var(--space-2); }
h3 { margin: 0; font-size: var(--text-base); font-weight: 800; }
.count { padding: 2px 8px; border-radius: var(--radius-pill); background: var(--kv-primary-soft); color: var(--kv-primary-soft-text); font-size: var(--text-xs); font-weight: var(--weight-bold); }
.see-all { margin-left: auto; border: 0; background: none; padding: 0; font: inherit; font-size: var(--text-sm); cursor: pointer; }
.muted { margin: 0; color: var(--kv-text-muted); font-size: var(--text-sm); }
.task-list { display: grid; gap: var(--space-3); margin: 0; padding: 0; list-style: none; }
.card { display: grid; grid-template-columns: 44px minmax(0, 1fr); gap: var(--space-2) var(--space-3); align-items: center; padding: var(--space-3); border: 1px solid var(--kv-line); border-radius: 16px; box-shadow: 0 3px 0 var(--kv-line); }
.ring .track { fill: none; stroke: var(--kv-line); stroke-width: 4; }
.ring .fill { fill: none; stroke: var(--kv-primary); stroke-width: 4; stroke-linecap: round; transform: rotate(-90deg); transform-origin: center; }
.ring text { font-size: 11px; font-weight: 800; fill: var(--kv-ink); }
.text { min-width: 0; display: grid; }
.text strong { font-size: var(--text-md); font-weight: 800; overflow-wrap: anywhere; }
.text small { color: var(--kv-text-muted); font-size: var(--text-sm); font-weight: var(--weight-semibold); }
.foot { grid-column: 1 / -1; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-2); margin: 0; }
.due { font-size: var(--text-sm); font-weight: var(--weight-semibold); color: var(--kv-text-muted); }
.due.soon { padding: 2px 8px; border-radius: 8px; background: var(--kv-learn-due-soft); color: var(--kv-learn-due-ink); }
.start { border-color: var(--kv-primary); color: var(--kv-primary-soft-text); }
</style>
