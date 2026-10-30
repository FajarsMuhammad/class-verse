<script setup lang="ts">
import Phaser from 'phaser'
import { onBeforeUnmount, onMounted, useTemplateRef } from 'vue'

import { CampusScene, type SceneDeps } from '@/game/CampusScene'

const props = defineProps<{ deps: SceneDeps; label: string }>()
const host = useTemplateRef<HTMLDivElement>('host')
let game: Phaser.Game | null = null

onMounted(() => {
  if (!host.value) return
  game = new Phaser.Game({
    type: Phaser.AUTO,
    parent: host.value,
    backgroundColor: 'transparent',
    scale: { mode: Phaser.Scale.RESIZE, width: '100%', height: '100%' },
    render: { antialias: true, roundPixels: false },
    banner: false,
    audio: { noAudio: true },
    input: { keyboard: { target: host.value } },
    scene: [],
  })
  game.scene.add('campus', CampusScene, true, props.deps)
})

onBeforeUnmount(() => {
  game?.destroy(true)
  game = null
})
</script>

<template>
  <div
    ref="host"
    class="canvas"
    role="application"
    :aria-label="label"
    tabindex="0"
    @pointerdown="host?.focus()"
  />
</template>

<style scoped>
.canvas {
  width: 100%;
  height: 100%;
  border-radius: var(--radius-2xl);
  overflow: hidden;
}

.canvas:focus-visible {
  outline: 3px solid var(--color-primary-strong);
  outline-offset: 2px;
}
</style>
