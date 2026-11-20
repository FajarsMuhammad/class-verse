<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef, useTemplateRef, watch } from 'vue'
import type { PDFDocumentLoadingTask, PDFDocumentProxy, RenderTask, TextLayer } from 'pdfjs-dist'
import workerURL from 'pdfjs-dist/build/pdf.worker.min.mjs?url'

const props = defineProps<{ data: Blob; startPage: number }>()
const emit = defineEmits<{ page: [page: number] }>()

const canvas = useTemplateRef<HTMLCanvasElement>('canvas')
const text = useTemplateRef<HTMLDivElement>('text')
const frame = useTemplateRef<HTMLDivElement>('frame')
const doc = shallowRef<PDFDocumentProxy | null>(null)
const page = ref(1)
const pages = ref(0)
const zoom = ref(1)
const error = ref('')
let render: RenderTask | null = null
let layer: TextLayer | null = null
let pdfjs: typeof import('pdfjs-dist') | null = null
let loading: PDFDocumentLoadingTask | null = null
let unmounted = false

const cancelled = (err: unknown) =>
  ['RenderingCancelledException', 'AbortException'].includes((err as { name?: string }).name ?? '')

// pdf.js rejects two render() calls on one canvas, so draws run one at a time;
// a newer request cancels the running one and the latest state wins.
let queue: Promise<void> = Promise.resolve()
function draw(): Promise<void> {
  render?.cancel()
  layer?.cancel()
  queue = queue.then(paint, paint)
  return queue
}

async function paint() {
  const pdf = doc.value
  if (!pdf || !canvas.value || !text.value || !pdfjs || unmounted) return
  const p = await pdf.getPage(page.value)
  // Fit the scroll frame's width (minus scrollbar room), then apply the user's zoom.
  const width = Math.max(240, (frame.value?.clientWidth ?? 800) - 16)
  const base = p.getViewport({ scale: 1 })
  const viewport = p.getViewport({ scale: (width / base.width) * zoom.value })
  const ratio = window.devicePixelRatio || 1
  canvas.value.width = Math.floor(viewport.width * ratio)
  canvas.value.height = Math.floor(viewport.height * ratio)
  canvas.value.style.width = `${Math.floor(viewport.width)}px`
  canvas.value.style.height = `${Math.floor(viewport.height)}px`
  const ctx = canvas.value.getContext('2d')
  if (!ctx) return
  ctx.setTransform(ratio, 0, 0, ratio, 0, 0)
  render = p.render({ canvas: canvas.value, canvasContext: ctx, viewport })
  text.value.replaceChildren()
  text.value.style.setProperty('--total-scale-factor', String(viewport.scale))
  layer = new pdfjs.TextLayer({ textContentSource: p.streamTextContent(), container: text.value, viewport })
  try {
    await Promise.all([render.promise, layer.render()])
  } catch (err) {
    if (!cancelled(err)) throw err
  }
}

function go(next: number) {
  if (!pages.value) return
  page.value = Math.min(pages.value, Math.max(1, next))
}

onMounted(async () => {
  try {
    pdfjs = await import('pdfjs-dist')
    pdfjs.GlobalWorkerOptions.workerSrc = workerURL
    const data = new Uint8Array(await props.data.arrayBuffer())
    if (unmounted) return
    loading = pdfjs.getDocument({ data })
    doc.value = await loading.promise
    pages.value = doc.value.numPages
    const start = Math.min(pages.value, Math.max(1, props.startPage))
    // Setting page triggers the watcher's draw; draw directly only when it does not change.
    if (start === page.value) await draw()
    else page.value = start
  } catch (err) {
    if (unmounted) return
    const name = (err as { name?: string }).name
    error.value =
      name === 'PasswordException'
        ? 'PDF ini berpassword dan tidak bisa dibuka.'
        : 'PDF tidak bisa dibuka. File mungkin rusak.'
  }
})

let resize: ResizeObserver | null = null
let lastWidth = 0
watch(frame, (el) => {
  resize?.disconnect()
  if (!el) return
  // Phones rotate and dialogs settle after first paint; redraw only on real width changes.
  resize = new ResizeObserver(([entry]) => {
    const w = Math.round(entry?.contentRect.width ?? 0)
    if (w && Math.abs(w - lastWidth) > 8) {
      lastWidth = w
      void draw().catch(() => (error.value = 'Halaman gagal ditampilkan.'))
    }
  })
  resize.observe(el)
})

watch([page, zoom], () => {
  emit('page', page.value)
  void draw().catch(() => (error.value = 'Halaman gagal ditampilkan.'))
})

onBeforeUnmount(() => {
  render?.cancel()
  layer?.cancel()
  unmounted = true
  resize?.disconnect()
  // Destroying the loading task also terminates the worker and frees the document.
  void loading?.destroy()
})
</script>

<template>
  <div class="pdf">
    <p v-if="error" role="alert" class="alert">{{ error }}</p>
    <template v-else>
      <div class="bar" role="toolbar" aria-label="Navigasi PDF">
        <button type="button" class="btn-secondary" :disabled="page <= 1" @click="go(page - 1)">‹ Sebelumnya</button>
        <label class="jump">
          Halaman
          <input
            type="number"
            min="1"
            :max="pages || 1"
            :value="page"
            inputmode="numeric"
            @change="go(Number(($event.target as HTMLInputElement).value))"
          />
          dari {{ pages || '…' }}
        </label>
        <button type="button" class="btn-secondary" :disabled="!pages || page >= pages" @click="go(page + 1)">Berikutnya ›</button>
        <button type="button" class="btn-secondary" aria-label="Perkecil" :disabled="zoom <= 0.5" @click="zoom = Math.max(0.5, zoom - 0.25)">−</button>
        <span aria-live="polite">{{ Math.round(zoom * 100) }}%</span>
        <button type="button" class="btn-secondary" aria-label="Perbesar" :disabled="zoom >= 3" @click="zoom = Math.min(3, zoom + 0.25)">+</button>
      </div>
      <p class="sr" aria-live="polite">Halaman {{ page }} dari {{ pages }}</p>
      <div ref="frame" class="page" @keydown.left="go(page - 1)" @keydown.right="go(page + 1)" tabindex="0" aria-label="Isi halaman PDF">
        <div class="sheet">
          <canvas ref="canvas" aria-hidden="true" />
          <div ref="text" class="textLayer" />
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-3);
}
.bar button,
.jump input {
  min-height: var(--touch-min);
  min-width: 44px;
}
.jump input {
  width: 5em;
}
.page {
  overflow: auto;
  max-height: 80vh;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface-alt);
}
.page:focus-visible {
  outline: 3px solid var(--color-primary-strong);
}
.sheet {
  position: relative;
  width: max-content;
  margin: 0 auto;
}
/* Minimal subset of pdfjs-dist/web/pdf_viewer.css: invisible but selectable, screen-reader text. */
.sheet :deep(.textLayer) {
  position: absolute;
  inset: 0;
  overflow: clip;
  line-height: 1;
  transform-origin: 0 0;
  --min-font-size: 1;
  --text-scale-factor: calc(var(--total-scale-factor) * var(--min-font-size));
  --min-font-size-inv: calc(1 / var(--min-font-size));
}
.sheet :deep(.textLayer :is(span, br)) {
  color: transparent;
  position: absolute;
  white-space: pre;
  cursor: text;
  transform-origin: 0% 0%;
}
.sheet :deep(.textLayer > :not(.markedContent)),
.sheet :deep(.textLayer .markedContent span:not(.markedContent)) {
  --font-height: 0;
  font-size: calc(var(--text-scale-factor) * var(--font-height));
  --scale-x: 1;
  --rotate: 0deg;
  transform: rotate(var(--rotate)) scaleX(var(--scale-x)) scale(var(--min-font-size-inv));
}
.sheet :deep(.textLayer .markedContent) {
  display: contents;
}
.sheet :deep(.textLayer ::selection) {
  background: var(--color-primary-soft);
}
</style>
