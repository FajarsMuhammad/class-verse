<script setup lang="ts">
defineProps<{ page: number; more: boolean; busy?: boolean }>()
const emit = defineEmits<{ move: [delta: number] }>()
</script>
<template><nav class="remote-pagination" aria-label="Halaman daftar"><button type="button" class="btn-secondary" :disabled="busy || page === 1" @click="emit('move', -1)">Sebelumnya</button><span>Halaman {{ page }}</span><button type="button" class="btn-secondary" :disabled="busy || !more" @click="emit('move', 1)">Berikutnya</button></nav></template>
<style scoped>.remote-pagination { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-3); margin-block: var(--space-3); }</style>
