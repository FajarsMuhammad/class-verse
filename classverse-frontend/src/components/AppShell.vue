<script setup lang="ts">
import { RouterLink, useRouter } from 'vue-router'
import { computed, nextTick, ref, useTemplateRef } from 'vue'
import ProfileDialog from './ProfileDialog.vue'

import logoUrl from '@/assets/avatars/round.svg'
import { useAuthStore } from '@/stores/auth'
import { useOrgStore } from '@/stores/org'

const props = defineProps<{ immersive?: boolean }>()

const auth = useAuthStore()
const org = useOrgStore()
const router = useRouter()
// Map workspace (teacher and participant): no top header; brand and account live in the side rail.
const mapWorkspace = computed(() => props.immersive &&
  !(typeof window.matchMedia === 'function' && window.matchMedia('(pointer: coarse) and (max-width: 767px)').matches))
const profileButton = useTemplateRef<HTMLButtonElement>('profileButton')
const profileOpen = ref(false)
async function closeProfile() {
  profileOpen.value = false
  await nextTick()
  profileButton.value?.focus()
}
const profileStatus = ref('')

async function logout() {
  await auth.logout()
  await router.push({ name: 'login' })
}
</script>

<template>
  <div class="shell" :class="{ immersive, 'map-workspace': mapWorkspace }">
    <header v-if="!mapWorkspace" class="top">
      <div class="brand">
        <img :src="logoUrl" alt="" width="40" height="40" />
        <span class="wordmark">Kelasverse</span>
        <RouterLink
          v-if="org.active"
          class="school"
          :to="{ name: 'select-org' }"
          :aria-label="`Sekolah aktif: ${org.active.name}. Ganti sekolah`"
        >
          {{ org.active.name }}
        </RouterLink>
      </div>
      <nav class="nav" aria-label="Navigasi utama">
        <RouterLink class="nav-link" :to="{ name: 'home' }">Peta sekolah</RouterLink>
        <RouterLink v-if="org.active" class="nav-link" :to="{ name: 'gradebook' }">Rapor</RouterLink>
        <RouterLink v-if="org.isTeacher" class="nav-link" :to="{ name: 'manage-rooms' }">
          Kelola sekolah
        </RouterLink>
        <RouterLink v-if="org.isTeacher" class="nav-link" :to="{ name: 'moderation' }">
          Moderasi
        </RouterLink>
      </nav>
      <div class="user">
        <button
          ref="profileButton"
          type="button"
          class="btn-secondary small"
          @click="profileOpen = true"
        >
          Profil
        </button>
        <span class="name">{{ auth.me?.display_name }}</span>
        <button type="button" class="btn-secondary small" @click="logout">Keluar</button>
      </div>
    </header>
    <ProfileDialog
      v-if="profileOpen"
      @close="closeProfile"
      @saved="profileStatus = 'Profil berhasil disimpan.'"
    />
    <p v-if="profileStatus" role="status" class="profile-status">{{ profileStatus }}</p>
    <main class="content">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.profile-status {
  margin: var(--space-2) var(--space-8);
  color: var(--color-primary-strong);
}
.shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.top {
  display: flex;
  align-items: center;
  gap: var(--space-6);
  padding: var(--space-3) var(--space-8);
  background: var(--color-surface);
  border-bottom: 1px solid var(--kv-line);
  flex-wrap: wrap;
}

.brand {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.brand img {
  width: var(--space-10);
  height: var(--space-10);
}

.wordmark {
  font-family: var(--font-display);
  font-weight: var(--weight-semibold);
  font-size: var(--text-xl);
}

.school {
  min-height: var(--touch-min);
  display: inline-flex;
  align-items: center;
  padding: 0 var(--space-4);
  border-radius: var(--radius-pill);
  background: var(--color-surface-alt);
  color: var(--color-ink);
  font-weight: var(--weight-bold);
  text-decoration: none;
}

.nav {
  display: flex;
  gap: var(--space-2);
  margin-left: auto;
}

.nav-link {
  min-height: var(--touch-min);
  display: inline-flex;
  align-items: center;
  padding: 0 var(--space-4);
  border-radius: var(--radius-md);
  color: var(--color-ink);
  font-weight: var(--weight-bold);
  text-decoration: none;
}

.nav-link.router-link-exact-active {
  background: var(--color-primary-soft);
  color: var(--color-primary-strong);
}

.school:focus-visible,
.nav-link:focus-visible {
  outline: 3px solid var(--color-primary-strong);
  outline-offset: 2px;
}

.user {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.name {
  font-weight: var(--weight-bold);
}

.small {
  min-height: var(--touch-min);
}

/* Phones: brand + account on one row, navigation as one scrollable row. */
@media (max-width: 767px) {
  .top {
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
  }
  .wordmark,
  .name {
    display: none;
  }
  .brand {
    flex: 1;
    min-width: 0;
  }
  .school {
    min-width: 0;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: block;
    line-height: var(--touch-min);
  }
  .user {
    gap: var(--space-2);
  }
  .nav {
    order: 3;
    flex-basis: 100%;
    margin-left: 0;
    overflow-x: auto;
  }
  .nav-link {
    white-space: nowrap;
  }
  .small {
    padding: 0 var(--space-3);
    font-size: var(--text-md);
  }
}
.immersive {
  height: 100dvh;
}
.immersive .content {
  max-width: none;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: var(--space-3);
}
.map-workspace .content { padding: var(--space-1); }
.content {
  flex: 1;
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  padding: var(--space-8);
  box-sizing: border-box;
}
</style>
