<script setup lang="ts">
import { useId } from 'vue'

defineProps<{
  label: string
  type?: string
  autocomplete?: string
  inputmode?: 'text' | 'numeric' | 'email'
  maxlength?: number
  error?: string
  hint?: string
}>()

const model = defineModel<string>({ required: true })
const id = useId()
</script>

<template>
  <div class="field">
    <label :for="id" class="label">{{ label }}</label>
    <input
      :id="id"
      v-model="model"
      class="input"
      :class="{ 'input--error': error }"
      :type="type ?? 'text'"
      :autocomplete="autocomplete"
      :inputmode="inputmode"
      :maxlength="maxlength"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="error || hint ? `${id}-desc` : undefined"
    />
    <p v-if="error" :id="`${id}-desc`" class="error">{{ error }}</p>
    <p v-else-if="hint" :id="`${id}-desc`" class="hint">{{ hint }}</p>
  </div>
</template>

<style scoped>
.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.label {
  font-weight: var(--weight-bold);
  font-size: var(--text-md);
}

.input {
  min-height: var(--touch-min);
  padding: 0 var(--space-3);
  border: 2px solid var(--color-input-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-ink);
  font: inherit;
  font-size: var(--text-base);
}

.input:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 1px;
  border-color: var(--color-primary);
}

.input--error {
  border-color: var(--color-accent-line);
}

.error,
.hint {
  margin: 0;
  font-size: var(--text-sm);
}

.error {
  color: var(--color-accent-ink);
  font-weight: var(--weight-semibold);
}

.hint {
  color: var(--color-ink-muted);
}
</style>
