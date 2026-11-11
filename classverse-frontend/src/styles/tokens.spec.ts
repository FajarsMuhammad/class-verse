/// <reference types="node" />
// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const css = readFileSync(new URL('./tokens.css', import.meta.url), 'utf8')

describe('MVP 2 design tokens', () => {
  it('uses local design tokens and fonts without external dependencies', () => {
    expect(css).toContain('--kv-primary: #5b4ee0;')
    expect(css).toContain("--kv-font-ui: 'Plus Jakarta Sans', system-ui, sans-serif;")
    expect(css).not.toMatch(/@import|https?:\/\//)
    expect(css).toContain("url('../assets/fonts/PlusJakartaSans-Variable.ttf')")
    expect(css).toContain('--font-body: var(--kv-font-ui)')
    expect(css).toContain('--color-primary: var(--kv-primary)')
    expect(css).toContain('--touch-min: 44px')
  })
})
