import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

// Phaser needs a real canvas; replace it with the parts GameCanvas touches.
const lose = vi.fn<() => void>()
const created: FakeGame[] = []
class FakeRenderer {
  gl: { getExtension: (name: string) => unknown } | null = {
    getExtension: (name) => (name === 'WEBGL_lose_context' ? { loseContext: lose } : null),
  }
}
class FakeGame {
  config: { fps?: { smoothStep?: boolean }; transparent?: boolean; backgroundColor?: string }
  renderer = new FakeRenderer()
  private handlers: Record<string, (() => void)[]> = {}
  events = {
    once: (name: string, fn: () => void) => (this.handlers[name] ??= []).push(fn),
  }
  scene = { add: vi.fn<() => void>() }
  destroy = vi.fn<(removeCanvas: boolean) => void>(() => {
    // Phaser runs destroy at the end of the frame and then nulls renderer.gl.
    const gl = this.renderer.gl
    for (const fn of this.handlers.destroy ?? []) fn()
    this.renderer.gl = null
    void gl
  })
  constructor(config: { fps?: { smoothStep?: boolean }; transparent?: boolean; backgroundColor?: string }) {
    this.config = config
    created.push(this)
  }
}
vi.mock('phaser', () => ({
  default: {
    Game: FakeGame,
    AUTO: 0,
    Scale: { RESIZE: 3 },
    Renderer: { WebGL: { WebGLRenderer: FakeRenderer } },
    Core: { Events: { DESTROY: 'destroy' } },
  },
}))
vi.mock('@/game/CampusScene', () => ({ CampusScene: class {} }))

const { default: GameCanvas } = await import('./GameCanvas.vue')

describe('GameCanvas', () => {
  it('focuses the map on entry without taking keyboard input back from chat', async () => {
    const w = mount(GameCanvas, { attachTo: document.body, props: { deps: {} as never, label: 'Peta' } })
    expect(document.activeElement).toBe(w.element)
    const chat = document.createElement('textarea')
    document.body.append(chat)
    chat.focus()
    await w.vm.$nextTick()
    expect(document.activeElement).toBe(chat)
    w.unmount()
    chat.remove()
    created.length = 0
    lose.mockClear()
  })
  it('releases the WebGL context when unmounted (travel remounts it)', () => {
    for (let i = 0; i < 3; i++) {
      const w = mount(GameCanvas, { props: { deps: {} as never, label: 'Peta' } })
      w.unmount()
    }
    expect(created).toHaveLength(3)
    for (const g of created) {
      expect(g.destroy).toHaveBeenCalledWith(true)
      expect(g.config.fps?.smoothStep).toBe(false)
      // Phaser parses backgroundColor 'transparent' as black; only `transparent` clears it.
      expect(g.config.transparent).toBe(true)
      expect(g.config.backgroundColor).toBeUndefined()
    }
    expect(lose).toHaveBeenCalledTimes(3)
  })
})
