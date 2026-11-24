<script setup lang="ts">
import { computed } from 'vue'
import RichContent from '../RichContent.vue'
import { embedURL, kindLabel } from '@/api/modules'
import type { PlayerStep } from './place'

// One step player for class tasks and building modules. Tasks are strictly ordered (locked steps cannot be
// opened); module steps are all open. The parent owns data and saving; this only shows and emits.
const props = defineProps<{
  mode: 'tugas' | 'modul'
  // id of the title, for the popup's aria-labelledby
  titleId: string
  steps: PlayerStep[]
  currentId: string
  title: string
  eyebrow: string
  subject: string
  color: string
  meta: string[]
  backLabel: string
  busy?: boolean
  readonly?: boolean
  error?: string
}>()
const emit = defineEmits<{
  select: [id: string]; complete: [id: string]; undo: [id: string]
  openFile: [step: PlayerStep]; startQuiz: [step: PlayerStep]; back: []
}>()
const index = computed(() => Math.max(0, props.steps.findIndex((s) => s.id === props.currentId)))
const current = computed(() => props.steps[index.value])
const done = computed(() => props.steps.filter((s) => s.done).length)
const locked = (s: PlayerStep | undefined) => !s || s.unlocked === false
const next = computed(() => props.steps[index.value + 1])
const status = (s: PlayerStep) => s.done ? 'Selesai' : locked(s) ? 'Terkunci' : s.id === props.currentId ? 'Sedang dibuka' : 'Belum'
</script>

<template>
  <header class="head">
    <div class="lead">
      <p class="eyebrow"><span class="chip" :style="{ '--tone': color }">{{ subject }}</span>{{ eyebrow }}</p>
      <h2 :id="titleId">{{ title }}</h2>
      <p class="meta"><span v-for="m in meta" :key="m">{{ m }}</span></p>
    </div>
    <div class="progress">
      <p><span>Progres</span><strong>{{ done }} dari {{ steps.length }} selesai</strong></p>
      <div role="progressbar" aria-label="Progres" aria-valuemin="0" :aria-valuemax="steps.length" :aria-valuenow="done">
        <span :style="{ width: `${steps.length ? (done / steps.length) * 100 : 0}%` }" />
      </div>
    </div>
    <button type="button" class="btn-secondary back" @click="emit('back')">{{ backLabel }}</button>
  </header>

  <div class="body">
    <nav class="steps" :aria-label="mode === 'tugas' ? 'Langkah tugas' : 'Langkah modul'">
      <h3>Langkah</h3>
      <button v-for="(s, i) in steps" :key="s.id" type="button" class="step" :class="{ current: s.id === currentId, done: s.done }"
        :disabled="locked(s)" :aria-current="s.id === currentId ? 'step' : undefined" @click="emit('select', s.id)">
        <span class="badge" aria-hidden="true">
          <svg v-if="s.done" viewBox="0 0 24 24" width="18" height="18"><path d="M20 6 9 17l-5-5" /></svg>
          <svg v-else-if="locked(s)" viewBox="0 0 24 24" width="16" height="16"><rect x="4" y="11" width="16" height="10" rx="2" /><path d="M8 11V7a4 4 0 0 1 8 0v4" /></svg>
          <template v-else>{{ i + 1 }}</template>
        </span>
        <span class="text"><strong>{{ s.title }}</strong><small>{{ kindLabel[s.kind] }} · {{ status(s) }}</small></span>
      </button>
      <p class="note">{{ mode === 'tugas'
        ? 'Langkah berikutnya terbuka setelah langkah sebelumnya selesai. Progres tugas terpisah dari katalog; nilai dan progres lama tetap tersimpan.'
        : 'Semua langkah terbuka. Tandai selesai agar progres muncul di Lanjutkan belajar.' }}</p>
    </nav>

    <main v-if="current" class="main">
      <div class="content">
        <p class="kind"><span>{{ kindLabel[current.kind] }}</span>Langkah {{ index + 1 }} dari {{ steps.length }} · {{ current.minutes }} menit</p>
        <h3 class="step-title">{{ current.title }}</h3>
        <p v-if="readonly" class="notice">Tugas dibatalkan. Riwayat tetap bisa dibaca.</p>
        <div v-if="current.kind === 'video' && current.video_id" class="video">
          <iframe :src="embedURL(current.video_id)" :title="`Video: ${current.title}`" loading="lazy"
            allow="encrypted-media; picture-in-picture; fullscreen" referrerpolicy="strict-origin-when-cross-origin"
            sandbox="allow-scripts allow-same-origin allow-presentation" />
        </div>
        <div v-if="current.kind === 'berkas' && current.file_id" class="file">
          <span class="file-icon" aria-hidden="true">{{ current.file_kind === 'audio' ? 'MP3' : 'PDF' }}</span>
          <div><strong>{{ current.title }}</strong><small>{{ current.file_kind === 'audio' ? 'Audio' : 'Dokumen PDF' }} · posisi baca tersimpan</small></div>
          <button type="button" class="btn-secondary" @click="emit('openFile', current)">{{ current.file_kind === 'audio' ? 'Dengarkan' : 'Buka berkas' }}</button>
        </div>
        <div v-if="current.kind === 'kuis' && current.quiz_id" class="quiz">
          <span class="quiz-icon" aria-hidden="true"><i /><i /><i /><i /></span>
          <div>
            <strong>{{ current.title }}</strong>
            <p>{{ mode === 'tugas' ? 'Lulus kuis melalui tugas ini. Hasil kuis katalog tidak menyelesaikan tugas.' : 'Langkah ini selesai otomatis saat kamu lulus kuis.' }}</p>
          </div>
          <button type="button" class="quiz-start" :disabled="readonly" @click="emit('startQuiz', current)">{{ current.done ? 'Kuis lulus · main lagi' : 'Mulai kuis' }}</button>
        </div>
        <RichContent v-if="current.rich_content" class="rich" :content="current.rich_content" />
        <p v-else-if="current.body" class="plain">{{ current.body }}</p>
        <p v-if="error" class="alert" role="alert">{{ error }}</p>
      </div>
      <footer class="foot">
        <button type="button" class="btn-secondary nav" :disabled="index === 0" @click="emit('select', steps[index - 1]!.id)">
          <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path d="m15 18-6-6 6-6" /></svg>Sebelumnya
        </button>
        <span class="spacer" />
        <template v-if="current.done">
          <span class="done-badge"><svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path d="M20 6 9 17l-5-5" /></svg>Selesai</span>
          <button v-if="mode === 'modul' && current.kind !== 'kuis'" type="button" class="btn-secondary" :disabled="busy" @click="emit('undo', current.id)">Batal selesai</button>
        </template>
        <button v-else-if="current.kind !== 'kuis' && !readonly" type="button" class="btn-primary mark" :disabled="busy" @click="emit('complete', current.id)">
          {{ busy ? 'Menyimpan…' : 'Tandai selesai' }}
        </button>
        <button type="button" class="btn-secondary nav next" :disabled="!next || locked(next)" @click="emit('select', next!.id)">
          {{ next ? 'Lanjut' : 'Langkah terakhir' }}<svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path d="m9 18 6-6-6-6" /></svg>
        </button>
      </footer>
    </main>
    <main v-else class="main"><p class="content muted">Belum ada langkah.</p></main>
  </div>
</template>

<style scoped>
.head { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-5); padding: var(--space-5) calc(var(--space-6) + 60px) var(--space-5) var(--space-6); border-bottom: 1px solid var(--kv-line); }
.lead { flex: 999 1 380px; min-width: 0; display: grid; gap: var(--space-1); }
.eyebrow { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-2); margin: 0; font-size: var(--text-sm); font-weight: var(--weight-semibold); color: var(--kv-text-muted); }
.chip { padding: 3px 10px; border-radius: var(--radius-pill); background: color-mix(in srgb, var(--tone) 12%, white); color: color-mix(in srgb, var(--tone) 80%, black); font-weight: var(--weight-bold); }
h2 { margin: 0; font-size: 30px; line-height: 1.15; font-weight: 800; overflow-wrap: anywhere; }
.meta { display: flex; flex-wrap: wrap; gap: var(--space-1) var(--space-4); margin: 0; font-size: var(--text-md); font-weight: var(--weight-semibold); color: var(--kv-text-muted); }
.progress { flex: 1 1 220px; max-width: 280px; display: grid; gap: var(--space-2); }
.progress p { display: flex; justify-content: space-between; margin: 0; font-size: var(--text-md); font-weight: var(--weight-bold); }
.progress strong { color: var(--kv-primary); }
[role='progressbar'] { height: 10px; border-radius: var(--radius-pill); background: var(--kv-line); overflow: hidden; }
[role='progressbar'] span { display: block; height: 100%; border-radius: inherit; background: var(--kv-primary); transition: width .2s; }
.back { height: 44px; }

.body { flex: 1; min-height: 0; display: flex; flex-wrap: wrap; overflow: auto; }
.steps { flex: 1 1 260px; box-sizing: border-box; display: flex; flex-direction: column; gap: var(--space-2); padding: var(--space-5); background: var(--kv-learn-rail); border-right: 1px solid var(--kv-line); }
.steps h3 { margin: 0 0 var(--space-1) var(--space-2); font-size: var(--text-sm); font-weight: 800; color: var(--kv-text-muted); }
.step { display: flex; align-items: center; gap: var(--space-3); width: 100%; padding: var(--space-3); border: 2px solid transparent; border-radius: 16px; background: transparent; color: var(--color-ink); font: inherit; text-align: left; cursor: pointer; }
.step.current { border-color: var(--kv-primary); background: var(--color-surface); box-shadow: 0 6px 18px color-mix(in srgb, var(--kv-primary) 14%, transparent); }
.step:disabled { opacity: .55; cursor: not-allowed; }
.step:focus-visible, .back:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
.badge { flex: none; width: 36px; height: 36px; box-sizing: border-box; display: grid; place-items: center; border-radius: 50%; border: 2px solid var(--kv-learn-step-line); background: var(--color-surface); font-weight: 800; }
.badge svg { fill: none; stroke: currentColor; stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
.step.current .badge { border-color: var(--kv-primary); background: var(--kv-primary); color: var(--color-surface); }
.step.done .badge { border-color: var(--kv-learn-done); background: var(--kv-learn-done); color: var(--color-surface); }
.step:disabled .badge { border-color: var(--kv-line); background: var(--kv-line); color: var(--kv-text-muted); }
.text { min-width: 0; display: grid; gap: 2px; }
.text strong { font-size: 15px; line-height: 1.3; overflow-wrap: anywhere; }
.text small { font-size: var(--text-sm); font-weight: var(--weight-semibold); color: var(--kv-text-muted); }
.note { margin: auto var(--space-2) 0; padding-top: var(--space-4); font-size: var(--text-sm); line-height: var(--line-relaxed); color: var(--kv-text-muted); }

.main { flex: 999 1 520px; min-width: 0; display: flex; flex-direction: column; }
.content { flex: 1; box-sizing: border-box; max-width: 820px; display: flex; flex-direction: column; gap: var(--space-5); padding: 30px 44px 28px; }
.kind { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-2); margin: 0; font-size: var(--text-md); font-weight: var(--weight-bold); color: var(--kv-text-muted); }
.kind span { padding: 4px 10px; border-radius: 8px; background: var(--kv-primary-soft); color: var(--kv-primary-soft-text); }
.step-title { margin: 0; font-size: 32px; line-height: 1.15; font-weight: 800; overflow-wrap: anywhere; }
.rich, .plain { font-size: 17px; line-height: 1.7; color: var(--kv-learn-body); overflow-wrap: anywhere; }
.plain { margin: 0; white-space: pre-wrap; }
.video { width: 100%; aspect-ratio: 16 / 9; border-radius: 18px; overflow: hidden; background: var(--kv-ink); }
.video iframe { width: 100%; height: 100%; border: 0; }
.file { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-4); padding: 18px; border: 1px solid var(--kv-line); border-radius: 18px; }
.file > div { flex: 1 1 200px; min-width: 0; display: grid; gap: 2px; }
.file small { color: var(--kv-text-muted); font-weight: var(--weight-semibold); }
.file-icon { flex: none; width: 52px; height: 60px; display: grid; place-items: center; border-radius: 10px; background: var(--kv-danger-soft); color: var(--kv-danger-text); font-size: var(--text-sm); font-weight: 800; }
.quiz { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-6); padding: 26px; border-radius: 20px; background: var(--kv-arena-soft); }
.quiz > div { flex: 1 1 240px; min-width: 0; display: grid; gap: var(--space-2); }
.quiz strong { font-size: var(--text-xl); font-weight: 800; }
.quiz p { margin: 0; font-size: 15px; line-height: 1.6; color: var(--kv-learn-quiz-ink); }
.quiz-icon { flex: none; display: grid; grid-template-columns: 1fr 1fr; gap: 6px; width: 88px; height: 88px; box-sizing: border-box; padding: 14px; border-radius: 22px; background: var(--kv-learn-quiz); }
.quiz-icon i { border-radius: 6px; background: var(--kv-quiz-red); }
.quiz-icon i:nth-child(2) { background: var(--kv-quiz-blue); }
.quiz-icon i:nth-child(3) { background: var(--kv-quiz-yellow); }
.quiz-icon i:nth-child(4) { background: var(--kv-quiz-green); }
.quiz-start { height: 48px; padding: 0 22px; border: 0; border-radius: 14px; background: var(--kv-learn-quiz); color: var(--color-surface); font: inherit; font-size: var(--text-base); font-weight: 800; cursor: pointer; }
.quiz-start:disabled { opacity: .6; cursor: not-allowed; }
.quiz-start:focus-visible { outline: 3px solid var(--color-primary-strong); outline-offset: 2px; }
.foot { position: sticky; bottom: 0; display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-3); padding: 18px 44px; border-top: 1px solid var(--kv-line); background: var(--color-surface); }
.foot button { min-height: 48px; border-radius: 14px; font-size: 15px; }
.nav svg { fill: none; stroke: currentColor; stroke-width: 2.4; stroke-linecap: round; stroke-linejoin: round; }
.next:not(:disabled) { border-color: var(--kv-primary); color: var(--kv-primary-soft-text); }
.mark { padding: 0 22px; font-weight: 800; }
.spacer { flex: 1; }
.done-badge { display: inline-flex; align-items: center; gap: var(--space-2); height: 44px; padding: 0 14px; border-radius: 12px; background: var(--kv-learn-done-soft); color: var(--kv-learn-done-ink); font-size: 15px; font-weight: var(--weight-bold); }
.done-badge svg { fill: none; stroke: currentColor; stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
.muted { color: var(--kv-text-muted); }

@media (max-width: 860px) {
  .head { padding-right: calc(var(--space-3) + 56px); }
  .steps { flex-basis: 100%; border-right: 0; border-bottom: 1px solid var(--kv-line); }
  .content, .foot { padding-inline: var(--space-5); }
  h2 { font-size: 24px; }
  .step-title { font-size: 26px; }
}
</style>
