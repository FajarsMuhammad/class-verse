<script setup lang="ts">
import { onMounted, useTemplateRef } from 'vue'
import { embedURL, kindLabel } from '@/api/modules'
import type { ModuleStep } from '@/api/types'

// Text and video steps open here; file steps use FileReader. Body is plain text (no HTML is rendered).
defineProps<{ step: ModuleStep }>()
const emit = defineEmits<{ close: [] }>()
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
onMounted(() => dialog.value?.showModal())
</script>

<template>
  <dialog ref="dialog" class="viewer" aria-labelledby="step-title" @close="emit('close')">
    <header class="head">
      <div>
        <p class="kind">{{ kindLabel[step.kind] }} · {{ step.minutes }} menit</p>
        <h2 id="step-title">{{ step.title }}</h2>
      </div>
      <button type="button" class="btn-secondary close" aria-label="Tutup" @click="dialog?.close()">×</button>
    </header>
    <div v-if="step.kind === 'video' && step.video_id" class="video">
      <iframe :src="embedURL(step.video_id)" :title="`Video: ${step.title}`" loading="lazy"
        allow="encrypted-media; picture-in-picture; fullscreen" referrerpolicy="strict-origin-when-cross-origin"
        sandbox="allow-scripts allow-same-origin allow-presentation" />
    </div>
    <p v-if="step.body" class="body">{{ step.body }}</p>
  </dialog>
</template>

<style scoped>
.viewer {
  box-sizing: border-box;
  width: min(100% - var(--space-6), 760px);
  max-height: calc(100dvh - var(--space-6));
  padding: var(--space-5);
  border: 2px solid var(--color-border);
  border-radius: var(--radius-2xl);
  background: var(--color-surface);
  color: var(--color-ink);
}
.viewer::backdrop {
  background: var(--color-ink);
  opacity: 0.4;
}
.head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
}
.kind {
  margin: 0;
  color: var(--color-ink-muted);
  font-size: var(--text-sm);
}
h2 {
  margin: 0;
  font: var(--weight-semibold) var(--text-xl)/var(--line-tight) var(--font-display);
}
.close {
  flex: none;
  min-width: 44px;
  padding: 0;
}
.video {
  margin-top: var(--space-4);
  aspect-ratio: 16 / 9;
}
.video iframe {
  width: 100%;
  height: 100%;
  border: 0;
  border-radius: var(--radius-lg);
}
.body {
  margin: var(--space-4) 0 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  line-height: var(--line-normal, 1.6);
}
</style>
