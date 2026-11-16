<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { RoomHandle } from '@/realtime/useRoom'
import type { Module, Level } from '@/api/types'
import { moduleApi } from '@/api/modules'
import { errorMessage } from '@/api/client'
import BuildingModules from './BuildingModules.vue'
const props = defineProps<{ room: RoomHandle; orgId: string; staff: boolean }>()
const pin = computed(() => props.room.pin?.value ?? null)
const interior = computed(() => props.room.current.value?.map_key !== 'campus')
const modules = ref<Module[]>([])
const moduleID = ref('')
const stepID = ref('')
const opened = ref(false)
const choosing = ref(false)
const error = ref('')
const busy = ref(false)
const selected = computed(() => modules.value.find(m => m.id === moduleID.value))
watch(moduleID, () => { stepID.value = '' })
watch(() => props.room.current.value?.room_id, () => { opened.value = false; choosing.value = false })
async function choose() {
  busy.value = true; error.value = ''
  try { modules.value = (await moduleApi.list(props.orgId)).items.filter(m => m.published); choosing.value = true }
  catch (e) { error.value = errorMessage(e) }
  finally { busy.value = false }
}
function set() { props.room.pinLearning?.(moduleID.value, stepID.value || undefined); choosing.value = false }
</script>
<template>
  <section v-if="interior" class="pin" aria-label="Belajar bersama">
    <p v-if="pin"><strong>{{ pin.title }}</strong> disematkan pengajar.</p>
    <button v-if="pin" type="button" :disabled="room.status.value !== 'open'" @click="opened = true">Buka bersama</button>
    <p v-if="pin">Membuka konten, bukan pemutaran tersinkron. Progres katalog tetap bebas.</p>
    <button v-if="staff" type="button" :disabled="busy || room.status.value !== 'open'" @click="choose">Sematkan modul</button>
    <button v-if="staff && pin" type="button" @click="room.pinLearning?.('')">Lepas sematan</button>
    <p v-if="error" role="alert">{{ error }}</p>
    <form v-if="choosing" @submit.prevent="set">
      <label>Modul terbit <select v-model="moduleID" required><option value="" disabled>Pilih modul</option><option v-for="m in modules" :key="m.id" :value="m.id">{{ m.title }} · {{ m.level }}</option></select></label>
      <label>Langkah opsional <select v-model="stepID"><option value="">Seluruh modul</option><option v-for="s in selected?.steps" :key="s.id" :value="s.id">{{ s.title }}</option></select></label>
      <button type="submit" :disabled="!moduleID">Sematkan</button><button type="button" @click="choosing = false">Batal</button>
    </form>
    <BuildingModules v-if="opened && pin" :org-id="orgId" :building-key="pin.building_key" :building-name="pin.title" :initial-level="pin.level as Level" :module-id="pin.module_id" :step-id="pin.step_id" @close="opened = false" />
  </section>
</template>
<style scoped>
.pin { padding: var(--space-3); border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-surface); min-width: 0; }
label { display: flex; flex-direction: column; gap: var(--space-2); margin-block: var(--space-2); }
button, select { font: inherit; color: var(--color-ink); background: var(--color-surface); min-height: 44px; border: 1px solid var(--color-border); border-radius: var(--radius-md); max-width: 100%; padding: var(--space-2); }
button { cursor: pointer; margin: var(--space-1); }
</style>
