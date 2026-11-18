import { inject, onBeforeUnmount, type ComputedRef } from 'vue'
import { matchedRouteKey, onBeforeRouteLeave } from 'vue-router'

// Both shell panel changes and actual route navigation must preserve unsaved author work.
export function useAuthorLeave(dirty: ComputedRef<boolean>) {
  const mayLeave = () => !dirty.value || window.confirm('Perubahan belum tersimpan. Tinggalkan editor?')
  function panel(event: Event) { if (!mayLeave()) event.preventDefault() }
  function unload(event: BeforeUnloadEvent) { if (dirty.value) { event.preventDefault(); event.returnValue = '' } }
  window.addEventListener('kv-before-panel-change', panel)
  window.addEventListener('beforeunload', unload)
  if (inject(matchedRouteKey, undefined)) onBeforeRouteLeave(mayLeave)
  onBeforeUnmount(() => {
    window.removeEventListener('kv-before-panel-change', panel)
    window.removeEventListener('beforeunload', unload)
  })
  return mayLeave
}
