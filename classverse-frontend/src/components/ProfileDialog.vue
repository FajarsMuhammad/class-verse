<script setup lang="ts">
import { computed, onMounted, ref, useTemplateRef } from 'vue'
import { ApiError } from '@/api/client'
import { avatarAccessories, avatarLook, avatarPalette, avatarURL, lookKey } from '@/game/avatars'
import { useAuthStore } from '@/stores/auth'

const emit = defineEmits<{ close: []; saved: [] }>()
const auth = useAuthStore()
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
const name = ref(auth.me?.display_name ?? '')
const originalAvatar = auth.me?.avatar_key ?? 'default'
const initialLook = avatarLook(originalAvatar)
const skin = ref(initialLook.skin)
const hair = ref(initialLook.hair)
const shirt = ref(initialLook.shirt)
const accessory = ref(initialLook.accessory)
const accessoryColor = ref(initialLook.accessoryColor)
const avatarChanged = ref(false)
const avatar = computed(() => avatarChanged.value
  ? lookKey({ skin: skin.value, hair: hair.value, shirt: shirt.value, accessory: accessory.value, accessoryColor: accessoryColor.value })
  : originalAvatar)
const paletteGroups = computed(() => [
  { key: 'skin', label: 'Warna kulit', colors: avatarPalette.skin, value: skin },
  // A hijab covers the hair, so its colour choice is hidden while one is worn.
  ...(accessory.value === 1 ? [] : [{ key: 'hair', label: 'Warna rambut', colors: avatarPalette.hair, value: hair }]),
  { key: 'shirt', label: 'Warna pakaian', colors: avatarPalette.shirt, value: shirt },
  ...(accessory.value ? [{ key: 'accessory-color', label: 'Warna aksesori', colors: avatarPalette.accessory, value: accessoryColor }] : []),
])
const busy = ref(false)
const error = ref('')
const fields = ref<Record<string, string>>({})
onMounted(() => dialog.value?.showModal())

function close() {
  if (!busy.value) dialog.value?.close()
}
async function save() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  fields.value = {}
  try {
    const patch: { display_name?: string; avatar_key?: string } = {}
    if (name.value !== auth.me?.display_name) patch.display_name = name.value
    if (avatar.value !== auth.me?.avatar_key) patch.avatar_key = avatar.value
    await auth.updateProfile(patch)
    emit('saved')
    dialog.value?.close()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Profil gagal disimpan. Coba lagi.'
    if (e instanceof ApiError) fields.value = e.fields
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <dialog
    ref="dialog"
    aria-labelledby="profile-title"
    @close="emit('close')"
    @cancel="busy && $event.preventDefault()"
  >
    <form @submit.prevent="save" :aria-busy="busy">
      <header>
        <div>
          <h2 id="profile-title">Profil kamu</h2>
          <p>Nama dan avatar terlihat oleh peserta lain.</p>
        </div>
        <button
          class="btn-secondary"
          type="button"
          aria-label="Tutup profil"
          :disabled="busy"
          @click="close"
        >
          Tutup
        </button>
      </header>
      <div class="preview">
        <img
          :src="avatarURL(avatar)"
          alt="Preview avatar pilihan"
          width="56"
          height="76"
        /><strong>{{ name || 'Nama kamu' }}</strong>
      </div>
      <label for="profile-name">Nama tampilan</label>
      <input
        id="profile-name"
        v-model="name"
        required
        :disabled="busy"
        :aria-invalid="!!fields.display_name"
        aria-describedby="name-help name-error"
        autofocus
      />
      <p id="name-help" class="help">3–24 karakter. Gunakan nama yang sopan.</p>
      <p id="name-error" class="error">{{ fields.display_name }}</p>
      <fieldset :disabled="busy" @change="avatarChanged = true">
        <legend>Aksesori</legend>
        <div class="choices">
          <label v-for="item in avatarAccessories" :key="item.id" class="choice" :class="{ selected: accessory === item.id }">
            <input v-model="accessory" type="radio" name="avatar-accessory" :value="item.id" />
            <span>{{ item.label }}</span>
          </label>
        </div>
      </fieldset>
      <fieldset v-for="group in paletteGroups" :key="group.key" :disabled="busy" @change="avatarChanged = true">
        <legend>{{ group.label }}</legend>
        <div class="choices colors">
          <label v-for="(color, index) in group.colors" :key="color" class="choice"
            :class="{ selected: group.value.value === index }">
            <input v-model="group.value.value" type="radio" :name="`avatar-${group.key}`" :value="index"
              :aria-label="`${group.label} ${index + 1}`" />
            <span class="swatch" :style="{ background: color }" aria-hidden="true"></span>
            <span>{{ index + 1 }}</span>
          </label>
        </div>
      </fieldset>
      <p class="help">Kulit, rambut, pakaian, dan aksesori bisa dipilih terpisah.</p>
      <p class="error">{{ fields.avatar_key }}</p>
      <p v-if="error" role="alert" class="error">{{ error }}</p>
      <footer>
        <button type="button" class="btn-secondary" :disabled="busy" @click="close">Batal</button>
        <button type="submit" class="btn-primary" :disabled="busy">
          {{ busy ? 'Menyimpan…' : 'Simpan' }}
        </button>
      </footer>
    </form>
  </dialog>
</template>

<style scoped>
dialog {
  width: min(640px, calc(100vw - var(--space-8)));
  max-height: calc(100vh - var(--space-8));
  box-sizing: border-box;
  border: 1px solid var(--kv-line);
  border-radius: var(--radius-2xl);
  padding: var(--space-6);
  background: var(--color-surface);
  color: var(--color-ink);
}
dialog::backdrop {
  background: var(--color-ink);
  opacity: 0.45;
}
header,
footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
}
h2 {
  font: var(--weight-semibold) var(--text-2xl)/var(--line-tight) var(--font-display);
  margin: 0;
}
p {
  margin: var(--space-2) 0;
}
header p,
.help {
  color: var(--color-ink-muted);
  font-size: var(--text-sm);
}
.preview {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-4);
  padding: var(--space-4);
  margin: var(--space-4) 0;
  background: var(--color-bg);
  border-radius: var(--radius-lg);
}
label,
legend {
  font-weight: var(--weight-bold);
}
#profile-name {
  box-sizing: border-box;
  width: 100%;
  min-height: var(--touch-min);
  border: 1px solid var(--color-input-border);
  border-radius: var(--radius-md);
  padding: var(--space-3);
  font: inherit;
  margin-top: var(--space-2);
}
fieldset {
  border: 0;
  margin: var(--space-4) 0;
  padding: 0;
}
legend {
  margin-bottom: var(--space-3);
}
.choices {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-2);
}
.choice {
  display: grid;
  justify-items: center;
  gap: var(--space-1);
  padding: var(--space-2);
  border: 1px solid var(--kv-line);
  border-radius: var(--radius-md);
  cursor: pointer;
  font-size: var(--text-xs);
  background: var(--color-bg);
}
.preview img, .choice img {
  object-fit: contain;
}
.swatch {
  width: var(--space-6);
  height: var(--space-6);
  border: 1px solid var(--color-ink);
  border-radius: var(--radius-xs);
}
.choice { min-height: var(--touch-min); }
.choice input {
  justify-self: start;
  accent-color: var(--color-primary);
}
.selected {
  border-color: var(--color-primary);
  background: var(--color-primary-soft);
}
.choice:focus-within {
  outline: 2px solid var(--color-primary-strong);
  outline-offset: 2px;
}
.error {
  color: var(--color-ink);
  font-weight: var(--weight-bold);
}
footer {
  justify-content: flex-end;
  margin-top: var(--space-4);
}
@media (max-width: 540px) {
  .choices {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
