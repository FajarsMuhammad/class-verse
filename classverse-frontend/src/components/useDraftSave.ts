import { computed, onBeforeUnmount, ref, watch, type Ref } from 'vue'
import { ApiError, errorMessage } from '@/api/client'

const autosaveDelay = 1200
// One in-flight write. Changes made while saving stay dirty; failure never marks them saved.
export function useDraftSave<T>(value: Ref<T>, save: () => Promise<void>) {
  const baseline = ref('')
  const state = ref<'saved' | 'dirty' | 'saving' | 'failed' | 'conflict'>('saved')
  const error = ref('')
  const enabled = ref(false)
  const dirty = computed(() => enabled.value && JSON.stringify(value.value) !== baseline.value)
  let timer: ReturnType<typeof setTimeout> | undefined
  function reset() { baseline.value = JSON.stringify(value.value); state.value = 'saved'; error.value = ''; enabled.value = true }
  async function flush(force = false) {
    clearTimeout(timer)
    if ((!dirty.value && !force) || state.value === 'saving' || state.value === 'conflict') return
    const sent = JSON.stringify(value.value)
    state.value = 'saving'; error.value = ''
    try {
      await save()
      baseline.value = sent
      state.value = dirty.value ? 'dirty' : 'saved'
      if (dirty.value) timer = setTimeout(() => void flush(), autosaveDelay)
    } catch (err) {
      state.value = err instanceof ApiError && err.code === 'draft_conflict' ? 'conflict' : 'failed'
      error.value = errorMessage(err)
    }
  }
  watch(value, () => {
    if (!enabled.value || state.value === 'conflict' || state.value === 'saving') return
    clearTimeout(timer)
    state.value = dirty.value ? 'dirty' : 'saved'
    if (dirty.value) timer = setTimeout(() => void flush(), autosaveDelay)
  }, { deep: true })
  function guard(event: BeforeUnloadEvent) { if (dirty.value) { event.preventDefault(); event.returnValue = '' } }
  window.addEventListener('beforeunload', guard)
  onBeforeUnmount(() => { clearTimeout(timer); window.removeEventListener('beforeunload', guard) })
  const labels = { saved: 'Tersimpan', dirty: 'Belum tersimpan', saving: 'Menyimpan…', failed: 'Gagal menyimpan', conflict: 'Konflik versi' }
  return { state, error, dirty, enabled, reset, flush, label: computed(() => labels[state.value]), discard: () => { enabled.value = false; clearTimeout(timer) } }
}
