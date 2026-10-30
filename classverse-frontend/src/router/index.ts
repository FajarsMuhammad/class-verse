import { createRouter, createWebHistory, type RouteLocationNormalized } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import HomeView from '../views/HomeView.vue'

declare module 'vue-router' {
  interface RouteMeta {
    // requiresAuth: page needs a logged-in user.
    requiresAuth?: boolean
    // guestOnly: logged-in users are sent home (login, register).
    guestOnly?: boolean
  }
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'home', component: HomeView, meta: { requiresAuth: true } },
    {
      path: '/masuk',
      name: 'login',
      component: () => import('../views/auth/LoginView.vue'),
      meta: { guestOnly: true },
    },
    {
      path: '/daftar',
      name: 'register',
      component: () => import('../views/auth/RegisterView.vue'),
      meta: { guestOnly: true },
    },
    {
      path: '/verifikasi',
      name: 'verify',
      component: () => import('../views/auth/VerifyView.vue'),
      meta: { guestOnly: true },
    },
    {
      path: '/auth/callback',
      name: 'google-callback',
      component: () => import('../views/auth/GoogleCallbackView.vue'),
    },
    {
      path: '/lengkapi-profil',
      name: 'complete-profile',
      component: () => import('../views/auth/CompleteProfileView.vue'),
      meta: { requiresAuth: true },
    },
  ],
})

export async function authGuard(to: RouteLocationNormalized) {
  const auth = useAuthStore()
  if (to.name === 'google-callback') return true
  await auth.restore()
  if (to.meta.requiresAuth && !auth.loggedIn) {
    return { name: 'login', query: to.fullPath === '/' ? {} : { redirect: to.fullPath } }
  }
  if (to.meta.guestOnly && auth.loggedIn) return { name: 'home' }
  if (auth.loggedIn && auth.needsProfile && to.name !== 'complete-profile') {
    return { name: 'complete-profile' }
  }
  if (auth.loggedIn && !auth.needsProfile && to.name === 'complete-profile') {
    return { name: 'home' }
  }
  return true
}

router.beforeEach(authGuard)

export default router
