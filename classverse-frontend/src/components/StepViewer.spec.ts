import { mount } from '@vue/test-utils'
import { beforeAll, expect, it } from 'vitest'
import StepViewer from './StepViewer.vue'

beforeAll(() => {
  // jsdom lacks <dialog> methods.
  HTMLDialogElement.prototype.showModal ??= function (this: HTMLDialogElement) { this.open = true }
  HTMLDialogElement.prototype.close ??= function (this: HTMLDialogElement) { this.open = false; this.dispatchEvent(new Event('close')) }
})

const base = { id: 's', title: 'Bab', file_id: null, file_kind: null, minutes: 3, reader_position: 0, done: false }

it('renders text as plain text, never as HTML', () => {
  const w = mount(StepViewer, { props: { step: { ...base, kind: 'materi', body: '<img src=x onerror=alert(1)>Halo', video_id: null } } })
  expect(w.find('img').exists()).toBe(false)
  expect(w.get('.body').text()).toBe('<img src=x onerror=alert(1)>Halo')
})

it('embeds YouTube through the privacy-enhanced domain in a sandboxed frame', () => {
  const w = mount(StepViewer, { props: { step: { ...base, kind: 'video', body: '', video_id: 'dQw4w9WgXcQ' } } })
  const frame = w.get('iframe')
  expect(frame.attributes('src')).toBe('https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?rel=0')
  expect(frame.attributes('sandbox')).toContain('allow-scripts')
  expect(frame.attributes('title')).toBe('Video: Bab')
})
