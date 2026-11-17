<script setup lang="ts">
import { computed, onMounted, ref, useId, useTemplateRef } from 'vue'

import { errorMessage } from '@/api/client'
import { reportApi } from '@/api/reports'

const props = defineProps<{
  targetId: string
  targetName: string
  roomId: string
  messageId?: number
  messageText?: string
}>()
const emit = defineEmits<{ close: []; reported: [] }>()

const MAX = 1000
const reason = ref('')
const error = ref('')
const sending = ref(false)
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
const field = useId()
const remaining = computed(() => MAX - [...reason.value].length)

onMounted(() => dialog.value?.showModal())

async function submit() {
  if (!reason.value.trim() || remaining.value < 0 || sending.value) return
  sending.value = true
  error.value = ''
  try {
    await reportApi.create(
      props.messageId
        ? { target_user_id: props.targetId, message_id: props.messageId, reason: reason.value }
        : { target_user_id: props.targetId, room_id: props.roomId, reason: reason.value },
    )
    emit('reported')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <dialog ref="dialog" class="dialog" aria-labelledby="report-title" @close="emit('close')">
    <form method="dialog" @submit.prevent="submit">
      <h2 id="report-title">Laporkan {{ targetName }}</h2>
      <p v-if="messageText" class="quote">“{{ messageText }}”</p>
      <p class="note">Laporan dikirim ke pengajar dan admin sekolah ini, beserta namamu.</p>
      <label :for="field">Alasan</label>
      <textarea :id="field" v-model="reason" rows="4" required></textarea>
      <p class="count" :class="{ over: remaining < 0 }">Sisa {{ remaining }} karakter</p>
      <p v-if="error" role="alert" class="error">{{ error }}</p>
      <div class="actions">
        <button type="button" class="btn-secondary" @click="dialog?.close()">Batal</button>
        <button
          type="submit"
          class="btn-primary"
          :disabled="sending || !reason.trim() || remaining < 0"
        >
          Kirim laporan
        </button>
      </div>
    </form>
  </dialog>
</template>

<style scoped>
.dialog {
  width: min(480px, calc(100vw - var(--space-8)));
  border: 0;
  border-radius: var(--radius-2xl);
  padding: var(--space-6);
  background: var(--color-surface);
  color: var(--color-ink);
}
.dialog::backdrop {
  background: var(--color-ink);
  opacity: 0.4;
}
h2 {
  margin: 0 0 var(--space-3);
  font: var(--weight-semibold) var(--text-xl)/var(--line-tight) var(--font-display);
}
.quote {
  margin: 0 0 var(--space-3);
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-alt);
  border-radius: var(--radius-md);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.note,
.count {
  color: var(--color-ink-muted);
  font-size: var(--text-sm);
}
label {
  display: block;
  font-weight: var(--weight-bold);
  margin-bottom: var(--space-1);
}
textarea {
  width: 100%;
  box-sizing: border-box;
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-input-border);
  border-radius: var(--radius-md);
  font: inherit;
  resize: vertical;
}
textarea:focus-visible {
  outline: 3px solid var(--color-primary-strong);
  outline-offset: 1px;
}
.count {
  text-align: right;
  margin: var(--space-1) 0 0;
}
.count.over,
.error {
  color: var(--color-ink);
  font-weight: var(--weight-bold);
}
.actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-3);
  margin-top: var(--space-4);
}
</style>
