<script setup lang="ts">
import { onMounted, ref, useTemplateRef } from 'vue'
import { errorMessage } from '@/api/client'
import { roomApi } from '@/api/rooms'
import type { Room, RoomMap } from '@/api/types'
import type { Building } from '@/realtime/mapdata'

const props = defineProps<{ orgId: string; buildings: Building[]; room?: Room }>()
const emit = defineEmits<{ close: []; saved: [room: Room] }>()
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
const name = ref(props.room?.name ?? '')
const building = ref(props.room?.building_key ?? props.buildings[0]?.key ?? '')
const mapKey = ref<RoomMap>(props.room?.map_key ?? 'classroom')
const scheduled = ref(!!props.room?.scheduled_at)
const localTime = ref(toLocalInput(props.room?.scheduled_at))
const busy = ref(false)
const error = ref('')
const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone

function toLocalInput(iso?: string | null) {
  if (!iso) return ''
  const date = new Date(iso)
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
onMounted(() => dialog.value?.showModal())

async function save() {
  if (busy.value) return
  error.value = ''
  const normalized = name.value.normalize('NFC').trim()
  if ([...normalized].length < 3 || [...normalized].length > 60) {
    error.value = 'Nama ruang harus 3–60 karakter.'
    return
  }
  const date = scheduled.value ? new Date(localTime.value) : null
  if (date && !Number.isFinite(date.getTime())) {
    error.value = 'Isi tanggal dan waktu jadwal yang valid.'
    return
  }
  busy.value = true
  try {
    const input = { name: normalized, scheduled_at: date?.toISOString() ?? null }
    const saved = props.room
      ? await roomApi.update(props.room.id, input)
      : await roomApi.create(props.orgId, {
          ...input,
          building_key: building.value,
          map_key: mapKey.value,
        })
    emit('saved', saved)
    dialog.value?.close()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <dialog
    ref="dialog"
    aria-labelledby="room-form-title"
    @close="emit('close')"
    @cancel="busy && $event.preventDefault()"
  >
    <form @submit.prevent="save" :aria-busy="busy">
      <h2 id="room-form-title">{{ room ? 'Ubah ruang' : 'Buat ruang' }}</h2>
      <p>Kapasitas 40 orang. Peserta masuk melalui pintu gedung di peta sekolah.</p>
      <fieldset :disabled="busy">
        <label for="room-name">Nama ruang</label>
        <input id="room-name" v-model="name" required autofocus autocomplete="off" />
        <template v-if="!room">
          <label for="room-building">Gedung</label>
          <select id="room-building" v-model="building" required>
            <option v-for="b in buildings" :key="b.key" :value="b.key">{{ b.name }}</option>
          </select>
          <label for="room-map">Peta ruang</label>
          <select id="room-map" v-model="mapKey">
            <option value="classroom">Ruang kelas</option>
            <option value="garden">Taman diskusi</option>
          </select>
        </template>
        <label class="check"><input v-model="scheduled" type="checkbox" /> Jadwalkan ruang</label>
        <template v-if="scheduled">
          <label for="room-schedule">Tanggal dan waktu ({{ timeZone }})</label>
          <input id="room-schedule" v-model="localTime" type="datetime-local" required />
          <p>Pengajar dan admin boleh masuk lebih awal.</p>
        </template>
        <p v-else>Ruang bisa dimasuki sekarang.</p>
      </fieldset>
      <p v-if="error" role="alert" class="error">{{ error }}</p>
      <footer>
        <button type="button" class="btn-secondary" :disabled="busy" @click="dialog?.close()">
          Batal
        </button>
        <button class="btn-primary" :disabled="busy || !building">
          {{ busy ? 'Menyimpan…' : 'Simpan' }}
        </button>
      </footer>
    </form>
  </dialog>
</template>

<style scoped>
dialog {
  border: 2px solid var(--color-border);
  border-radius: var(--radius-2xl);
  padding: var(--space-6);
  background: var(--color-surface);
  color: var(--color-ink);
  width: min(100% - var(--space-8), 520px);
  box-sizing: border-box;
}
dialog::backdrop {
  background: var(--color-ink);
  opacity: 0.4;
}
h2 {
  margin: 0;
  font: var(--weight-semibold) var(--text-2xl)/var(--line-tight) var(--font-display);
}
p {
  color: var(--color-ink-muted);
}
fieldset {
  border: 0;
  padding: 0;
  margin: 0;
  display: grid;
  gap: var(--space-2);
}
label {
  font-weight: var(--weight-bold);
  margin-top: var(--space-2);
}
input:not([type='checkbox']),
select {
  width: 100%;
  min-height: var(--touch-min);
  padding: var(--space-2) var(--space-3);
  border: 2px solid var(--color-input-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-ink);
  font: inherit;
  box-sizing: border-box;
}
.check {
  display: flex;
  gap: var(--space-2);
  align-items: center;
  min-height: var(--touch-min);
}
footer {
  display: flex;
  justify-content: end;
  gap: var(--space-3);
  margin-top: var(--space-5);
}
.error {
  color: var(--color-ink);
  font-weight: var(--weight-bold);
}
</style>
