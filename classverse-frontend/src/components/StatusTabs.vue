<script setup lang="ts">
// Kelola list filter (docs/design/kelola handoff §2). Tabs filter one list; arrow keys move between tabs.
const props = defineProps<{ items: { key: string; label: string; count: number }[]; label: string; controls: string }>()
const active = defineModel<string>({ required: true })
function key(e: KeyboardEvent, index: number) {
  const delta = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0
  if (!delta) return
  const next = props.items[(index + delta + props.items.length) % props.items.length]!
  active.value = next.key
  ;((e.currentTarget as HTMLElement).parentElement?.children[props.items.indexOf(next)] as HTMLElement | undefined)?.focus()
}
</script>

<template>
  <div class="status-tabs" role="tablist" :aria-label="label">
    <button v-for="(item, i) in items" :key="item.key" type="button" role="tab" :aria-selected="active === item.key"
      :aria-controls="controls" :tabindex="active === item.key ? 0 : -1" @click="active = item.key" @keydown="key($event, i)">
      {{ item.label }} <span class="count">{{ item.count }}</span>
    </button>
  </div>
</template>

<style scoped>
.status-tabs { display: inline-flex; flex-wrap: wrap; gap: var(--space-1); padding: var(--space-1); border-radius: var(--control-radius); background: var(--kv-segment-bg); }
button { min-height: var(--touch-min); padding: 0 var(--space-3); border: 0; border-radius: var(--radius-md); background: transparent; color: var(--kv-text-muted); font: var(--weight-semibold) var(--text-sm)/1 var(--font-body); cursor: pointer; }
button[aria-selected='true'] { background: var(--kv-surface); color: var(--kv-ink); box-shadow: 0 0 0 1px var(--kv-line); }
button:focus-visible { outline: 2px solid var(--kv-primary); outline-offset: 2px; }
.count { margin-left: var(--space-1); font-size: var(--text-xs); }
</style>
