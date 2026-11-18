<script setup lang="ts">
import { computed, onMounted, useTemplateRef } from 'vue'
import RichContent from './RichContent.vue'
import { embedURL, kindLabel } from '@/api/modules'
import type { ModuleStep } from '@/api/types'

// Text and video steps open here; file steps use FileReader. Body is plain text (no HTML is rendered).
const props = defineProps<{ step: ModuleStep; steps?: ModuleStep[]; moduleTitle?: string; saving?: boolean }>()
const emit = defineEmits<{ close: []; select: [step: ModuleStep]; done: [step: ModuleStep] }>()
const index = computed(() => props.steps?.findIndex((s) => s.id === props.step.id) ?? -1)
function select(offset: number) {
  const step = props.steps?.[index.value + offset]
  if (step) emit('select', step)
}
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
onMounted(() => dialog.value?.showModal())
</script>

<template>
  <dialog ref="dialog" class="viewer" aria-labelledby="step-title" @close="emit('close')">
    <header class="head">
      <div>
        <p v-if="moduleTitle" class="module-title">{{ moduleTitle }}</p>
        <p class="kind">{{ kindLabel[step.kind] }} · {{ step.minutes }} menit</p>
        <h2 id="step-title">{{ step.title }}</h2>
      </div>
      <button type="button" class="btn-secondary close" aria-label="Tutup" @click="dialog?.close()">×</button>
    </header>
    <div v-if="steps?.length" class="segments" aria-label="Progres modul">
      <span v-for="s in steps" :key="s.id" :class="{ done: s.done, current: s.id === step.id }" />
    </div>
    <div class="player-layout">
      <nav v-if="steps?.length" class="contents" aria-label="Isi modul">
        <h3>Isi modul</h3>
        <button v-for="s in steps" :key="s.id" type="button" :aria-current="s.id === step.id ? 'step' : undefined"
          @click="emit('select', s)"><strong>{{ s.title }}</strong><small>{{ s.done ? 'Selesai' : `${kindLabel[s.kind]} · ${s.minutes} mnt` }}</small></button>
      </nav>
      <main class="content">
    <div v-if="step.kind === 'video' && step.video_id" class="video">
      <iframe :src="embedURL(step.video_id)" :title="`Video: ${step.title}`" loading="lazy"
        allow="encrypted-media; picture-in-picture; fullscreen" referrerpolicy="strict-origin-when-cross-origin"
        sandbox="allow-scripts allow-same-origin allow-presentation" />
    </div>
    <RichContent v-if="step.rich_content" :content="step.rich_content" />
    <p v-else-if="step.body" class="body">{{ step.body }}</p>
    <footer v-if="steps?.length" class="controls">
      <button type="button" class="btn-secondary" :disabled="index <= 0" @click="select(-1)">Sebelumnya</button>
      <button v-if="step.kind !== 'kuis'" type="button" class="btn-secondary" :disabled="saving" :aria-pressed="step.done" @click="emit('done', step)">{{ step.done ? 'Batal selesai' : 'Selesai' }}</button>
      <button type="button" class="btn-primary" :disabled="index >= steps.length - 1" @click="select(1)">Lanjut</button>
    </footer>
      </main>
    </div>
  </dialog>
</template>

<style scoped>
.viewer {
  box-sizing: border-box;
  width: min(100% - var(--space-6), 1280px);
  max-height: calc(100dvh - var(--space-6));
  padding: var(--space-5);
  border: 1px solid var(--kv-line);
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
.module-title { margin: 0 0 var(--space-2); color: var(--kv-subject-ipa); font-weight: var(--weight-semibold); }
.segments { display: flex; gap: var(--space-2); margin: var(--space-4) 0; }
.segments span { flex: 1; height: 8px; border-radius: var(--radius-pill); background: var(--color-border); }
.segments .done { background: var(--kv-subject-diskusi); }
.segments .current { background: var(--kv-subject-ipa); }
.player-layout { display: grid; grid-template-columns: 240px minmax(0, 1fr); gap: var(--space-5); }
.contents { display: grid; align-content: start; gap: var(--space-2); }
.contents h3 { margin: 0 0 var(--space-2); }
.contents button { display: grid; gap: var(--space-1); text-align: left; padding: var(--space-3); min-height: var(--touch-min); border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-surface); color: var(--color-ink); font: inherit; cursor: pointer; }
.contents button[aria-current] { border-color: var(--kv-subject-ipa); background: var(--color-primary-soft); }
.contents small { color: var(--color-ink-muted); }
.content { min-width: 0; border-radius: var(--radius-2xl); padding: var(--space-5); background: var(--color-surface-alt); }
.controls { display: flex; flex-wrap: wrap; justify-content: space-between; gap: var(--space-2); margin-top: var(--space-6); }
@media (max-width: 700px) { .player-layout { grid-template-columns: minmax(0, 1fr); } .contents { max-height: 180px; overflow-y: auto; } .content { padding: var(--space-3); } }
.body {
  margin: var(--space-4) 0 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  line-height: var(--line-normal, 1.6);
}
</style>
