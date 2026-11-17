<script setup lang="ts">
import { defineAsyncComponent, onBeforeUnmount, onMounted, ref, useTemplateRef } from 'vue'
import { errorMessage } from '@/api/client'
import { fetchContent, fileApi } from '@/api/files'
import type { FileItem } from '@/api/types'

// PDF.js (~1.7 MB with worker) loads only when a PDF is opened.
const PdfReader = defineAsyncComponent(() => import('./PdfReader.vue'))

// Only what the reader needs, so both file rows and catalog books can open it.
export type ReadableFile = Pick<FileItem, 'id' | 'name' | 'kind' | 'position'>
const props = defineProps<{ orgId: string; file: ReadableFile }>()
const emit = defineEmits<{ close: [position: number] }>()

const dialog = useTemplateRef<HTMLDialogElement>('dialog')
const audio = useTemplateRef<HTMLAudioElement>('audio')
const blob = ref<Blob | null>(null)
const url = ref('')
const error = ref('')
const controller = new AbortController()
let position = props.file.position
let timer: ReturnType<typeof setTimeout> | undefined

const SAVE_DELAY_MS = 1500

function save(next: number) {
  position = Math.max(0, Math.floor(next))
  clearTimeout(timer)
  timer = setTimeout(flush, SAVE_DELAY_MS)
}
function flush() {
  clearTimeout(timer)
  timer = undefined
  // Losing one position update is harmless; never block closing on it.
  fileApi.savePosition(props.orgId, props.file.id, position).catch(() => {})
}
function onAudioReady() {
  if (audio.value && position > 0 && position < (audio.value.duration || Infinity)) audio.value.currentTime = position
}
function close() {
  dialog.value?.close()
}
function onClose() {
  if (timer) flush()
  emit('close', position)
}

onMounted(async () => {
  dialog.value?.showModal()
  try {
    blob.value = await fetchContent(props.orgId, props.file.id, controller.signal)
    if (props.file.kind === 'audio') url.value = URL.createObjectURL(blob.value)
  } catch (err) {
    if (!controller.signal.aborted) error.value = errorMessage(err)
  }
})
onBeforeUnmount(() => {
  controller.abort()
  if (timer) flush()
  if (url.value) URL.revokeObjectURL(url.value)
})
</script>

<template>
  <dialog ref="dialog" class="reader" :aria-label="file.name" @close="onClose">
    <header class="head">
      <h2 class="title">{{ file.name }}</h2>
      <button type="button" class="btn-secondary" @click="close">Tutup</button>
    </header>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>
    <p v-else-if="!blob" role="status">Memuat file…</p>
    <PdfReader
      v-else-if="file.kind === 'pdf'"
      :data="blob"
      :start-page="Math.max(1, file.position)"
      @page="save"
    />
    <audio
      v-else-if="url"
      ref="audio"
      class="audio"
      controls
      controlslist="nodownload"
      preload="metadata"
      :src="url"
      @loadedmetadata="onAudioReady"
      @timeupdate="save(($event.target as HTMLAudioElement).currentTime)"
      @pause="flush"
    >
      Browser tidak mendukung pemutar audio.
    </audio>
  </dialog>
</template>

<style scoped>
.reader {
  box-sizing: border-box;
  width: min(960px, 100vw - var(--space-4));
  max-width: 100vw;
  max-height: calc(100dvh - var(--space-4));
  padding: var(--space-5);
  border: none;
  border-radius: var(--radius-xl);
  background: var(--color-surface);
  color: var(--color-ink);
}
.reader::backdrop {
  background: var(--color-ink);
  opacity: 0.45;
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}
.title {
  margin: 0;
  font-family: var(--font-display);
  font-size: var(--text-xl);
  overflow-wrap: anywhere;
}
.audio {
  width: 100%;
}
</style>
