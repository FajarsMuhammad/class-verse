import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { authApi, type Me } from '@/api/auth'
import { onTokenChange, refreshAccessToken, setAccessToken } from '@/api/client'

export const useAuthStore = defineStore('auth', () => {
  const me = ref<Me | null>(null)
  const restored = ref(false)
  let restoring: Promise<void> | null = null

  const loggedIn = computed(() => me.value !== null)
  const needsProfile = computed(() => me.value?.needs_profile ?? false)

  // The client drops the token when a refresh fails; mirror that here.
  onTokenChange((token) => {
    if (token === null) me.value = null
  })

  async function loadMe() {
    me.value = await authApi.me()
  }

  // restore recovers the session from the refresh cookie once per page load.
  function restore(): Promise<void> {
    if (!restoring) {
      restoring = (async () => {
        try {
          if (await refreshAccessToken()) await loadMe()
        } catch {
          me.value = null
        } finally {
          restored.value = true
        }
      })()
    }
    return restoring
  }

  async function startSession(accessToken: string) {
    setAccessToken(accessToken)
    await loadMe()
  }

  // completeGoogleLogin exchanges the cookie set by the Google callback for
  // an access token.
  async function completeGoogleLogin(): Promise<boolean> {
    const token = await refreshAccessToken()
    if (!token) return false
    await loadMe()
    return true
  }

  async function login(email: string, password: string) {
    const { access_token } = await authApi.login(email, password)
    await startSession(access_token)
  }

  async function verifyEmail(email: string, code: string) {
    const { access_token } = await authApi.verifyEmail(email, code)
    await startSession(access_token)
  }

  // reload fetches /api/me again, e.g. after joining or creating an organization.
  async function reload() {
    await loadMe()
  }

  async function updateProfile(patch: {
    display_name?: string
    birth_date?: string
    avatar_key?: string
  }) {
    me.value = await authApi.updateMe(patch)
  }

  async function logout() {
    try {
      await authApi.logout()
    } finally {
      setAccessToken(null)
      me.value = null
    }
  }

  return {
    me,
    restored,
    loggedIn,
    needsProfile,
    restore,
    login,
    verifyEmail,
    completeGoogleLogin,
    updateProfile,
    reload,
    logout,
  }
})
