<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import type { RoomHandle } from '@/realtime/useRoom'
import type { Module, Level } from '@/api/types'
import { moduleApi } from '@/api/modules'
import { errorMessage } from '@/api/client'
import BuildingModules from './BuildingModules.vue'

// Live-session pin (A1). Staff choose it from the Modul side-menu panel; participants see it at the top of
// Belajar. Opening is per person: it never drives anyone's screen or playback.
const props = defineProps<{ room: RoomHandle; orgId: string; staff: boolean }>()
const id = useId()
const pin = computed(() => props.room.pin?.value ?? null)
const online = computed(() => props.room.status.value === 'open')
const modules = ref<Module[]>([])
const moduleID = ref('')
const stepID = ref('')
const opened = ref(false)
const choosing = ref(false)
const error = ref('')
const busy = ref(false)
const selected = computed(() => modules.value.find((m) => m.id === moduleID.value))
const pinnedStep = computed(() => {
  const p = pin.value
  return p?.step_id ? modules.value.find((m) => m.id === p.module_id)?.steps.find((s) => s.id === p.step_id)?.title ?? '' : ''
})
watch(moduleID, () => { stepID.value = '' })
watch(() => props.room.current.value?.room_id, () => { opened.value = false; choosing.value = false })

async function choose() {
  busy.value = true
  error.value = ''
  try {
    modules.value = (await moduleApi.list(props.orgId)).items.filter((m) => m.published && m.steps.length)
    moduleID.value = pin.value?.module_id ?? ''
    stepID.value = pin.value?.step_id ?? ''
    choosing.value = true
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
function set() {
  props.room.pinLearning?.(moduleID.value, stepID.value || undefined)
  choosing.value = false
}
</script>

<template>
  <section class="pin-board" :aria-labelledby="`${id}-title`">
    <h3 :id="`${id}-title`" class="title">{{ staff ? 'Sematkan di ruang ini' : 'Dari pengajar' }}</h3>

    <article v-if="pin" class="pin-card">
      <svg class="pushpin" viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
        <path d="M9 3h6l-1 6 4 4H6l4-4zM12 13v8" />
      </svg>
      <div class="pin-text">
        <strong>{{ pin.title }}</strong>
        <span v-if="pinnedStep">Mulai dari {{ pinnedStep }}</span>
        <span v-else>Seluruh modul</span>
      </div>
      <button type="button" class="btn-primary open" :disabled="!online" @click="opened = true">Buka bersama</button>
    </article>
    <p class="note">
      {{ staff
        ? 'Peserta di ruang ini melihat tombol Buka bersama. Layar mereka tidak dipaksa berpindah.'
        : pin ? 'Membuka modul di layarmu sendiri. Progresmu tetap tersimpan.' : 'Belum ada modul yang disematkan.' }}
    </p>

    <div v-if="staff && !choosing" class="actions">
      <button type="button" class="btn-secondary" :disabled="busy || !online" @click="choose">
        {{ pin ? 'Ganti sematan' : 'Pilih modul' }}
      </button>
      <button v-if="pin" type="button" class="btn-secondary" :disabled="!online" @click="room.pinLearning?.('')">Lepas sematan</button>
    </div>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>

    <form v-if="staff && choosing" class="chooser" @submit.prevent="set">
      <label :for="`${id}-m`">Modul terbit</label>
      <select :id="`${id}-m`" v-model="moduleID" required>
        <option value="" disabled>Pilih modul</option>
        <option v-for="m in modules" :key="m.id" :value="m.id">{{ m.title }} ({{ m.level }})</option>
      </select>
      <label :for="`${id}-s`">Mulai dari langkah</label>
      <select :id="`${id}-s`" v-model="stepID" :disabled="!selected">
        <option value="">Seluruh modul</option>
        <option v-for="s in selected?.steps" :key="s.id" :value="s.id">{{ s.title }}</option>
      </select>
      <p v-if="!modules.length" class="note">Belum ada modul terbit. Terbitkan modul dulu di Ruang Guru.</p>
      <div class="actions">
        <button type="submit" class="btn-primary" :disabled="!moduleID || !online">Sematkan</button>
        <button type="button" class="btn-secondary" @click="choosing = false">Batal</button>
      </div>
    </form>

    <BuildingModules v-if="opened && pin" :org-id="orgId" :building-key="pin.building_key" :building-name="pin.title"
      :initial-level="pin.level as Level" :module-id="pin.module_id" :step-id="pin.step_id" @close="opened = false" />
  </section>
</template>

<style scoped>
.pin-board {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding-bottom: var(--space-4);
  border-bottom: 1px solid var(--kv-line);
}
.title {
  margin: 0;
  font: var(--weight-semibold) var(--text-md) / var(--line-tight) var(--kv-font-display);
}
/* The one bold element: a note pinned to the board, in the brand's nila. */
.pin-card {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: var(--space-2) var(--space-3);
  align-items: start;
  padding: var(--space-3) var(--space-4);
  background: var(--kv-primary-soft);
  border: 2px solid var(--kv-primary);
  border-radius: var(--kv-radius-card);
  transform: rotate(-0.6deg);
}
.pushpin {
  fill: var(--kv-primary);
  stroke: var(--kv-primary-soft-text);
  stroke-width: 1.6;
  stroke-linejoin: round;
  margin-top: 2px;
}
.pin-text { display: flex; flex-direction: column; gap: var(--space-1); min-width: 0; overflow-wrap: anywhere; }
.pin-text strong { font: var(--weight-semibold) var(--text-lg) / var(--line-tight) var(--kv-font-display); color: var(--kv-ink); }
.pin-text span { color: var(--kv-primary-soft-text); font-size: var(--text-sm); }
.open { grid-column: 1 / -1; justify-self: start; }
.note { margin: 0; color: var(--kv-text-muted); font-size: var(--text-sm); }
.alert { margin: 0; color: var(--kv-danger-text); }
.actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.chooser { display: flex; flex-direction: column; gap: var(--space-2); }
.chooser label { font-weight: var(--weight-semibold); font-size: var(--text-sm); }
select {
  min-height: var(--touch-min);
  max-width: 100%;
  padding: 0 var(--space-3);
  border: 1px solid var(--kv-line);
  border-radius: var(--kv-radius-input);
  background: var(--kv-surface);
  color: var(--kv-ink);
  font: inherit;
}
</style>
