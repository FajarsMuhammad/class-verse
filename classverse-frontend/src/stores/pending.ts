import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

const REDIRECT_KEY = 'kv-after-login'

// usePendingStore carries data between auth pages without putting it in the
// URL (URLs end up in history and logs): the email being verified and where
// to go after login (e.g. an invite link opened while logged out). The
// redirect path survives the full-page Google round trip via sessionStorage.
export const usePendingStore = defineStore('pending', () => {
  const email = ref('')
  const redirect = ref<string | null>(safeRedirect(readSession()))

  watch(
    redirect,
    (path) => {
      try {
        if (path) sessionStorage.setItem(REDIRECT_KEY, path)
        else sessionStorage.removeItem(REDIRECT_KEY)
      } catch {
        // storage unavailable; redirect lasts for this page only
      }
    },
    { flush: 'sync' },
  )

  // takeRedirect returns the stored in-app path once, then forgets it.
  function takeRedirect(): string | null {
    const r = redirect.value
    redirect.value = null
    return r
  }

  return { email, redirect, takeRedirect }
})

// safeRedirect accepts only same-app paths, so ?redirect= cannot send users
// to another site.
export function safeRedirect(value: unknown): string | null {
  if (typeof value !== 'string') return null
  if (!value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return null
  return value
}

function readSession(): string | null {
  try {
    return sessionStorage.getItem(REDIRECT_KEY)
  } catch {
    return null
  }
}
