import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'

import type { OrgSummary } from '@/api/types'
import { useAuthStore } from './auth'

const STORAGE_KEY = 'kv-active-org'

// useOrgStore tracks which organization the user is working in. Only the id
// is persisted; the server still checks membership on every request.
export const useOrgStore = defineStore('org', () => {
  const auth = useAuthStore()
  const activeId = ref<string | null>(readStored())

  const orgs = computed<OrgSummary[]>(() => auth.me?.orgs ?? [])
  const active = computed<OrgSummary | null>(() => {
    const list = orgs.value
    if (list.length === 1) return list[0] ?? null
    return list.find((o) => o.id === activeId.value) ?? null
  })
  const isTeacher = computed(
    () => active.value?.role === 'admin' || active.value?.role === 'teacher',
  )

  watch(activeId, (id) => {
    try {
      if (id) localStorage.setItem(STORAGE_KEY, id)
      else localStorage.removeItem(STORAGE_KEY)
    } catch {
      // storage unavailable (private mode); selection lasts for this tab only
    }
  })

  function select(id: string) {
    activeId.value = id
  }

  const isAdmin = computed(() => active.value?.role === 'admin')

  return { orgs, active, activeId, isTeacher, isAdmin, select }
})

function readStored(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}
