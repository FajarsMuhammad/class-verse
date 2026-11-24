<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { errorMessage } from '@/api/client'
import RemotePagination from './RemotePagination.vue'
export type SelectOption = { value: string; label: string; disabled?: boolean }
const props = defineProps<{ label: string; load: (offset: number) => Promise<{ items: SelectOption[]; more: boolean }>; placeholder?: string; disabled?: boolean; id?: string; testId?: string }>()
const value = defineModel<string>({ required: true })
const options = ref<SelectOption[]>([])
const selected = ref<SelectOption>()
const page = ref(1)
const more = ref(false)
const busy = ref(false)
const error = ref('')
let generation = 0
onBeforeUnmount(() => { generation++ })
async function fetchPage() {
  const g = ++generation
  busy.value = true; error.value = ''
  try {
    const result = await props.load((page.value - 1) * 20)
    if (g !== generation) return
    options.value = result.items; more.value = result.more
    const option = result.items.find(o => o.value === value.value)
    if (option) selected.value = option
  } catch (e) { if (g === generation) error.value = errorMessage(e) }
  finally { if (g === generation) busy.value = false }
}
function choose() { selected.value = options.value.find(o => o.value === value.value) ?? selected.value }
function move(delta: number) { page.value += delta; void fetchPage() }
watch(() => props.load, () => { page.value = 1; void fetchPage() })
onMounted(fetchPage)
</script>
<template>
  <div class="paged-select" :aria-busy="busy">
    <label>{{ label }}
      <select :data-test="testId" :id="id" :aria-label="label" v-model="value" :disabled="disabled || busy" @change="choose">
        <option value="">{{ placeholder || 'Pilih' }}</option>
        <option v-if="value && !options.some(o => o.value === value)" :value="value">{{ selected?.value === value ? selected.label : 'Pilihan tersimpan' }}</option>
        <option v-for="option in options" :key="option.value" :value="option.value" :disabled="option.disabled">{{ option.label }}</option>
      </select>
    </label>
    <p v-if="error" role="alert">{{ error }} <button type="button" class="btn-secondary" @click="fetchPage">Coba lagi</button></p>
    <RemotePagination v-if="page > 1 || more" :page="page" :more="more" :busy="busy || disabled" @move="move" />
  </div>
</template>
<style scoped>
.paged-select { min-width: 0; }
label { display: grid; gap: var(--space-2); font-size: var(--text-sm); font-weight: var(--weight-semibold); }
select { width: 100%; min-width: 0; min-height: var(--touch-min); padding: var(--space-2); border: 1px solid var(--kv-line); border-radius: var(--radius-md); background: var(--color-surface); color: var(--color-ink); font: inherit; }
</style>
