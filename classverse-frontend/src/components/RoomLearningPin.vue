<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import type { RoomHandle } from '@/realtime/useRoom'
import type { Level } from '@/api/types'
import { moduleApi } from '@/api/modules'
import { errorMessage } from '@/api/client'
import BuildingModules from './BuildingModules.vue'

// Session status belongs to the room panel. Selecting a module belongs to its detail editor.
const props = defineProps<{ room: RoomHandle; orgId: string; staff: boolean }>()
const emit = defineEmits<{ change: [] }>()
const id = useId()
const pin = computed(() => props.room.pin?.value ?? null)
const online = computed(() => props.room.status.value === 'open')
const opened = ref(false)
const stepTitle = ref('')
const error = ref('')
watch(() => props.room.current.value?.room_id, () => { opened.value = false })
watch([pin, () => props.orgId], async ([value], _, onCleanup) => {
  let stale = false
  onCleanup(() => { stale = true })
  stepTitle.value = ''
  error.value = ''
  if (!value?.step_id) return
  try {
    const result = await moduleApi.list(props.orgId, { building_key: value.building_key })
    if (!stale) stepTitle.value = result.items.find((m) => m.id === value.module_id)?.steps.find((s) => s.id === value.step_id)?.title ?? ''
  } catch (err) {
    if (!stale) error.value = errorMessage(err)
  }
}, { immediate: true })
</script>

<template>
  <section class="pin-board" :aria-labelledby="`${id}-title`">
    <h3 :id="`${id}-title`">{{ staff ? 'Materi sesi ini' : 'Dari pengajar' }}</h3>
    <template v-if="pin">
      <div class="pin-card">
        <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path d="M9 3h6l-1 6 4 4H6l4-4zM12 13v8" /></svg>
        <div class="pin-text">
          <strong>{{ pin.title }}</strong>
          <span>{{ pin.step_id ? stepTitle || 'Materi pilihan pengajar' : 'Seluruh modul' }}</span>
        </div>
      </div>
      <div class="actions">
        <button type="button" class="btn-primary open" :disabled="!online" @click="opened = true">Buka materi</button>
        <template v-if="staff">
          <button type="button" class="btn-secondary" :disabled="!online" @click="emit('change')">Ganti</button>
          <button type="button" class="btn-secondary" :disabled="!online" @click="room.pinLearning?.('')">Lepas</button>
        </template>
      </div>
      <p class="note">{{ staff ? 'Peserta membuka materi di layar masing-masing.' : 'Membuka materi di layarmu sendiri. Progresmu tetap tersimpan.' }}</p>
    </template>
    <template v-else-if="staff">
      <p class="note">Belum ada materi sesi. Pilih modul terbit dari detail modul.</p>
      <button type="button" class="btn-secondary choose" :disabled="!online" @click="emit('change')">Pilih modul</button>
    </template>
    <p v-else class="note">Belum ada modul yang disematkan.</p>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>
    <BuildingModules v-if="opened && pin" :org-id="orgId" :building-key="pin.building_key" :building-name="pin.title"
      :initial-level="pin.level as Level" :module-id="pin.module_id" :step-id="pin.step_id" @close="opened = false" />
  </section>
</template>

<style scoped>
.pin-board { display: flex; flex-direction: column; gap: var(--space-2); padding-bottom: var(--space-3); margin-bottom: var(--space-3); border-bottom: 1px solid var(--kv-line); }
h3 { margin: 0; font-size: var(--text-sm); font-weight: var(--weight-semibold); }
.pin-card { display: flex; align-items: flex-start; gap: var(--space-2); padding: var(--space-3); border: 1px solid var(--kv-line); border-radius: var(--radius-lg); background: var(--color-primary-soft); }
svg { flex: none; fill: none; stroke: var(--color-primary-strong); stroke-width: 1.6; stroke-linejoin: round; }
.pin-text { display: flex; flex-direction: column; gap: var(--space-1); min-width: 0; overflow-wrap: anywhere; }
.pin-text strong { font-size: var(--text-md); font-weight: var(--weight-semibold); }
.pin-text span, .note { color: var(--kv-text-muted); font-size: var(--text-sm); }
.note, .alert { margin: 0; }
.actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.choose { align-self: flex-start; }
</style>
