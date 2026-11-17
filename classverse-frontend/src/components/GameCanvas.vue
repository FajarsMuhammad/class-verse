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
    // backgroundColor: 'transparent' parses as black in Phaser 3.90.
    transparent: true,
    scale: { mode: Phaser.Scale.RESIZE, width: '100%', height: '100%' },
    render: { antialias: true, roundPixels: false },
    // Use elapsed time for server-synced movement, not Phaser's smoothed 60 Hz delta.
    fps: { smoothStep: false },
    banner: false,
    audio: { noAudio: true },
    input: { keyboard: { target: host.value } },
    scene: [],
  })
  game.scene.add('campus', CampusScene, true, props.deps)
  // Do not steal focus from chat or forms when a room change remounts the map.
  const active = document.activeElement
  if (!active || active === document.body || !active.isConnected) host.value.focus({ preventScroll: true })
})

// releaseContext frees the GPU context. Phaser's destroy drops its references
// but never calls WEBGL_lose_context, so every travel (which remounts this
// component) leaked one context until Chrome's limit of 16 forced the oldest
// ones to be lost: the stutter after entering and leaving rooms.
function releaseContext(g: Phaser.Game) {
  const renderer = g.renderer
  if (!(renderer instanceof Phaser.Renderer.WebGL.WebGLRenderer)) return
  const gl = renderer.gl
  // Phaser destroys at the end of the current frame and nulls renderer.gl.
  g.events.once(Phaser.Core.Events.DESTROY, () => {
    gl?.getExtension('WEBGL_lose_context')?.loseContext()
  })
}

onBeforeUnmount(() => {
  if (game) {
    releaseContext(game)
    game.destroy(true)
  }
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
