<script setup lang="ts">
const props = defineProps<{ total: number; size: number; label: string }>()
const page = defineModel<number>({ required: true })
</script>
<template>
  <nav v-if="total > size" class="list-pagination" :aria-label="label">
    <span>{{ (page - 1) * size + 1 }}–{{ Math.min(page * size, total) }} dari {{ total }}</span>
    <button type="button" class="btn-secondary" :disabled="page <= 1" @click="page--">Sebelumnya</button>
    <span>Halaman {{ page }} dari {{ Math.ceil(props.total / size) }}</span>
    <button type="button" class="btn-secondary" :disabled="page * size >= total" @click="page++">Berikutnya</button>
  </nav>
</template>
<style scoped>
.list-pagination { display: flex; flex-wrap: wrap; gap: var(--space-3); align-items: center; padding-block: var(--space-3); font-size: var(--text-sm); color: var(--kv-text-muted); }
</style>
