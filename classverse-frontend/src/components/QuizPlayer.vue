<script setup lang="ts">
import { computed, onMounted, ref, useTemplateRef } from 'vue'
import { errorMessage } from '@/api/client'
import { quizApi, shapes } from '@/api/quizzes'
import type { QuizAttempt, QuizQuestion } from '@/api/types'

// Q1 self-paced quiz. The server grades; the key and explanations arrive only after finishing
// (approved rule 3). Questions are plain text, never HTML.
const props = defineProps<{ orgId: string; quizId: string; title: string; assignmentStepId?: string; moduleId?: string; stepId?: string; previewQuestions?: QuizQuestion[]; passScore?: number }>()
const emit = defineEmits<{ close: []; finished: [attempt: QuizAttempt] }>()

const dialog = useTemplateRef<HTMLDialogElement>('dialog')
const attempt = ref<QuizAttempt | null>(null)
const current = ref(0)
const busy = ref(false)
const error = ref('')

const question = computed(() => attempt.value?.questions[current.value])
const unanswered = computed(() => attempt.value?.answers.filter((a) => a < 0).length ?? 0)

async function start() {
  busy.value = true
  error.value = ''
  try {
    if (props.previewQuestions) {
      attempt.value = { id: 'preview', quiz_id: props.quizId, questions: props.previewQuestions.map(q => ({ prompt: q.prompt, options: q.options, answer: null, explanation: null })), answers: props.previewQuestions.map(() => -1), pass_score: props.passScore ?? 60, finished: false, correct: null, score: null, passed: null }
      return
    }
    attempt.value = props.assignmentStepId
      ? await quizApi.start(props.orgId, props.quizId, props.assignmentStepId)
      : await quizApi.start(props.orgId, props.quizId, undefined, props.moduleId, props.stepId)
    current.value = Math.max(0, attempt.value.answers.findIndex((a) => a < 0))
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}

async function choose(choice: number) {
  const a = attempt.value
  if (!a || a.finished || busy.value) return
  busy.value = true
  error.value = ''
  const index = current.value
  try {
    if (!props.previewQuestions) await quizApi.answer(props.orgId, props.quizId, a.id, index, choice)
    attempt.value = { ...a, answers: a.answers.map((v, i) => (i === index ? choice : v)) }
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}

async function finish() {
  const a = attempt.value
  if (!a || busy.value) return
  if (unanswered.value && !window.confirm(`${unanswered.value} soal belum dijawab dan dihitung salah. Selesaikan kuis?`)) return
  busy.value = true
  error.value = ''
  try {
    if (props.previewQuestions) {
      const correct = props.previewQuestions.filter((q, i) => q.answer === a.answers[i]).length
      const score = Math.floor(correct * 100 / Math.max(1, props.previewQuestions.length))
      attempt.value = { ...a, questions: props.previewQuestions, finished: true, correct, score, passed: score >= a.pass_score }
    } else attempt.value = await quizApi.finish(props.orgId, props.quizId, a.id)
    current.value = 0
    if (!props.previewQuestions) emit('finished', attempt.value)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}

async function retry() {
  attempt.value = null
  await start()
}

onMounted(() => {
  dialog.value?.showModal()
  void start()
})
</script>

<template>
  <dialog ref="dialog" class="quiz" aria-labelledby="quiz-title" @close="emit('close')">
    <p v-if="previewQuestions" role="status">Simulasi kuis — Hasil tidak disimpan</p>
    <header class="head">
      <h2 id="quiz-title">{{ title }}</h2>
      <button type="button" class="btn-secondary close" aria-label="Tutup kuis" @click="dialog?.close()">×</button>
    </header>
    <p v-if="error" class="alert" role="alert">{{ error }} <button v-if="!attempt" type="button" class="link" @click="start">Coba lagi</button></p>
    <p v-if="!attempt && busy" role="status">Menyiapkan kuis…</p>

    <template v-if="attempt && !attempt.finished && question">
      <div class="progress" aria-hidden="true">
        <button v-for="(a, i) in attempt.answers" :key="i" type="button" tabindex="-1"
          :class="{ answered: a >= 0, current: i === current }" @click="current = i" />
      </div>
      <p class="counter" role="status">Soal {{ current + 1 }} dari {{ attempt.questions.length }}</p>
      <h3 class="prompt">{{ question.prompt }}</h3>
      <div class="options" role="group" :aria-label="`Pilihan jawaban soal ${current + 1}`">
        <button v-for="(text, i) in question.options" :key="i" type="button" :class="['option', `shape-${i}`]"
          :aria-pressed="attempt.answers[current] === i" :aria-label="`${shapes[i]!.name}: ${text}`" :disabled="busy"
          @click="choose(i)">
          <span class="shape" aria-hidden="true">{{ shapes[i]!.symbol }}</span>
          <span class="text">{{ text }}</span>
        </button>
      </div>
      <footer class="controls">
        <button type="button" class="btn-secondary prev" :disabled="current === 0" @click="current--">Sebelumnya</button>
        <button type="button" class="btn-secondary next" :disabled="current >= attempt.questions.length - 1" @click="current++">Berikutnya</button>
        <button type="button" class="btn-primary finish" :disabled="busy" @click="finish">Selesaikan kuis</button>
      </footer>
    </template>

    <section v-if="attempt?.finished" class="result" aria-labelledby="result-title">
      <h3 id="result-title" :class="['score', { passed: attempt.passed }]">
        Nilai {{ attempt.score }} · {{ attempt.passed ? 'Lulus' : 'Belum lulus' }}
      </h3>
      <p class="muted">{{ attempt.correct }} dari {{ attempt.questions.length }} benar · nilai lulus {{ attempt.pass_score }}</p>
      <ol class="review">
        <li v-for="(q, i) in attempt.questions" :key="i" :class="{ wrong: attempt.answers[i] !== q.answer }">
          <strong>{{ q.prompt }}</strong>
          <span>Jawabanmu: {{ attempt.answers[i]! >= 0 ? q.options[attempt.answers[i]!] : 'tidak dijawab' }}
            {{ attempt.answers[i] === q.answer ? '✓' : '✗' }}</span>
          <span v-if="attempt.answers[i] !== q.answer">Jawaban benar: {{ q.options[q.answer!] }}</span>
          <p v-if="q.explanation" class="explanation">{{ q.explanation }}</p>
        </li>
      </ol>
      <footer class="controls">
        <button type="button" class="btn-secondary" :disabled="busy" @click="retry">Coba lagi</button>
        <button type="button" class="btn-primary" @click="dialog?.close()">Tutup</button>
      </footer>
    </section>
  </dialog>
</template>

<style scoped>
.quiz {
  box-sizing: border-box;
  width: min(100% - var(--space-6), 760px);
  max-height: calc(100dvh - var(--space-6));
  padding: var(--space-5);
  border: 1px solid var(--kv-line);
  border-radius: var(--radius-2xl);
  box-shadow: var(--kv-shadow-card);
  background: var(--kv-learning-surface);
  color: var(--color-ink);
}
.quiz::backdrop {
  background: var(--color-ink);
  opacity: 0.4;
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
}
h2,
h3 {
  margin: 0;
  font-family: var(--font-display);
}
.close {
  flex: none;
  min-width: 44px;
  padding: 0;
}
.progress {
  display: flex;
  gap: var(--space-1);
  margin-top: var(--space-4);
}
.progress button {
  flex: 1;
  height: 8px;
  padding: 0;
  border: none;
  border-radius: var(--radius-pill);
  background: var(--color-surface-alt);
  cursor: pointer;
}
.progress .answered { background: var(--kv-subject-ipa); }
.progress .current { outline: 2px solid var(--kv-ink); }
.counter,
.muted {
  margin: var(--space-3) 0 0;
  color: var(--color-ink-muted);
}
.prompt {
  margin: var(--space-2) 0 var(--space-4);
  font-size: var(--text-xl);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-3);
}
.option {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-height: 64px;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--kv-line);
  border-radius: var(--radius-lg);
  box-shadow: var(--kv-shadow-card);
  color: var(--kv-surface);
  font: inherit;
  font-weight: var(--weight-semibold);
  text-align: left;
  cursor: pointer;
}
.shape-0 { background: var(--kv-subject-arena); }
.shape-1 { background: var(--kv-subject-matematika); }
.shape-2 { background: var(--kv-subject-ipa); }
.shape-3 { background: var(--kv-subject-kreatif); }
.option[aria-pressed='true'] {
  outline: 4px solid var(--kv-star);
  outline-offset: 2px;
}
.option:focus-visible {
  outline: 4px solid var(--color-primary-strong);
  outline-offset: 2px;
}
.option:disabled { cursor: progress; }
.shape { font-size: var(--text-xl); }
.text { min-width: 0; overflow-wrap: anywhere; }
.controls {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  justify-content: flex-end;
  margin-top: var(--space-5);
}
.controls button { min-height: var(--touch-min); }
.finish { margin-left: auto; }
.result .score { margin-top: var(--space-4); font-size: var(--text-2xl); color: var(--kv-star-text); }
.result .score.passed { color: var(--kv-subject-ipa); }
.review {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  margin: var(--space-4) 0 0;
  padding-left: var(--space-5);
}
.review li {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  overflow-wrap: anywhere;
}
.review strong { white-space: pre-wrap; }
.explanation {
  margin: 0;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  background: var(--color-surface-alt);
  white-space: pre-wrap;
}
@media (max-width: 560px) {
  .options { grid-template-columns: minmax(0, 1fr); }
  .finish { margin-left: 0; }
}
</style>
