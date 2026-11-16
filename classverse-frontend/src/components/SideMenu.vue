<script setup lang="ts">
import { computed, nextTick, useId, useTemplateRef, watch } from 'vue'
import { RouterLink } from 'vue-router'

// Gather-style side menu (docs/screens/sidemenu.png): a thin icon rail that is always visible and one
// collapsible panel beside the map. `keep` items stay mounted while hidden (chat keeps its draft/scroll);
// other items mount only while open, so heavy staff tools fetch nothing until used.
export interface SideItem {
  key: string
  label: string
  icon: string
  keep?: boolean
  wide?: boolean
  dot?: boolean
}
const props = defineProps<{ items: SideItem[]; label: string; link?: { to: string; label: string; icon: string } }>()
const active = defineModel<string | null>('active', { default: null })
const current = computed(() => props.items.find((i) => i.key === active.value) ?? null)

const id = useId()
const heading = useTemplateRef<HTMLElement>('heading')
const rail = useTemplateRef<HTMLElement>('rail')
// Only user actions move focus; the room opening a panel on entry must not steal it from the map.
let userAction = false
function toggle(key: string) {
  userAction = true
  active.value = active.value === key ? null : key
}
function close() {
  userAction = true
  active.value = null
}
// Escape inside a native dialog (reader, quiz) belongs to that dialog, not to the panel.
function onEscape(e: KeyboardEvent) {
  if (e.defaultPrevented || (e.target as Element | null)?.closest('dialog')) return
  close()
}
// Focus follows the user: into the panel title on open, back to the rail button on close.
watch(active, async (now, before) => {
  if (!userAction) return
  userAction = false
  await nextTick()
  if (now && before !== now) heading.value?.focus()
  else if (!now && before) rail.value?.querySelector<HTMLElement>(`[data-item="${before}"]`)?.focus()
})
</script>

<template>
  <div class="side-menu" :class="{ open: !!current, wide: current?.wide }" @keydown.esc="onEscape">
    <nav ref="rail" class="rail" :aria-label="label">
      <button v-for="item in items" :key="item.key" type="button" class="rail-button" :data-item="item.key"
        :aria-pressed="active === item.key" :aria-controls="`${id}-panel`"
        :aria-label="item.dot ? `${item.label} (ada pesan baru)` : undefined" @click="toggle(item.key)">
        <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true"><path :d="item.icon" /></svg>
        <span>{{ item.label }}</span>
        <i v-if="item.dot" class="dot" aria-hidden="true" />
      </button>
      <RouterLink v-if="link" class="rail-button" :to="link.to">
        <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true"><path :d="link.icon" /></svg>
        <span>{{ link.label }}</span>
      </RouterLink>
    </nav>
    <section v-show="current" :id="`${id}-panel`" class="panel" :aria-labelledby="`${id}-title`">
      <header class="panel-head">
        <h2 :id="`${id}-title`" ref="heading" tabindex="-1">{{ current?.label }}</h2>
        <button type="button" class="btn-secondary collapse" aria-label="Ciutkan menu samping" @click="close">‹</button>
      </header>
      <div v-for="item in items" v-show="active === item.key" :key="item.key" class="panel-body" :data-panel="item.key">
        <slot v-if="item.keep || active === item.key" :name="item.key" />
      </div>
    </section>
  </div>
</template>

<style scoped>
.side-menu {
  display: flex;
  gap: var(--space-3);
  min-height: 0;
}
.rail {
  flex: none;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  width: 64px;
  box-sizing: border-box;
  padding: var(--space-2) var(--space-1);
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
  overflow-y: auto;
  scrollbar-width: none;
}
.rail-button {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-1);
  min-height: 52px;
  padding: var(--space-2) 0;
  border: 0;
  border-radius: var(--radius-lg);
  background: transparent;
  color: var(--color-ink);
  font: var(--weight-semibold) var(--text-xs) / 1.1 var(--font-body);
  text-align: center;
  text-decoration: none;
  cursor: pointer;
}
.rail-button svg { fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
.rail-button:hover, .rail-button[aria-pressed='true'] { background: var(--color-primary-soft); color: var(--color-primary-strong); }
.rail-button:focus-visible, .panel-head h2:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
.dot { position: absolute; top: 6px; right: 12px; width: 10px; height: 10px; border-radius: 50%; background: var(--color-danger, #d83a52); }
.panel {
  display: flex;
  flex-direction: column;
  width: 320px;
  min-height: 0;
  box-sizing: border-box;
  padding: var(--space-4);
  background: var(--color-surface);
  border-radius: var(--radius-2xl);
}
.wide .panel { width: min(640px, 46vw); }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); margin-bottom: var(--space-3); }
.panel-head h2 { margin: 0; font: var(--weight-semibold) var(--text-lg) / var(--line-tight) var(--font-display); }
.collapse { flex: none; min-width: var(--touch-min, 44px); padding: 0; }
.panel-body { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow-y: auto; }
@media (max-width: 900px) {
  .side-menu { flex-direction: column; }
  .rail { flex-direction: row; width: auto; overflow-x: auto; }
  .rail-button { flex: none; min-width: 64px; }
  .panel, .wide .panel { width: auto; flex: 1; }
}
</style>
