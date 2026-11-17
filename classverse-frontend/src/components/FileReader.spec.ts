import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import FileReader from './FileReader.vue'
import { ApiError } from '@/api/client'
import * as files from '@/api/files'
import type { FileItem } from '@/api/types'

const audio: FileItem = {
  id: 'f1',
  name: 'Lagu.mp3',
  kind: 'audio',
  content_type: 'audio/mpeg',
  size: 3,
  uploaded_by: 'u',
  created_at: '2026-01-01T00:00:00Z',
  position: 42,
}

beforeEach(() => {
  vi.useFakeTimers()
  HTMLDialogElement.prototype.showModal = vi.fn<() => void>()
  HTMLDialogElement.prototype.close = vi.fn<() => void>(function (this: HTMLDialogElement) {
    this.dispatchEvent(new Event('close'))
  })
  URL.createObjectURL = vi.fn<(b: Blob) => string>(() => 'blob:audio')
  URL.revokeObjectURL = vi.fn<(u: string) => void>()
})
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

it('plays private audio from a blob URL, resumes, and saves position on close', async () => {
  vi.spyOn(files, 'fetchContent').mockResolvedValue(new Blob(['abc']))
  const save = vi.spyOn(files.fileApi, 'savePosition').mockResolvedValue()
  const wrapper = mount(FileReader, { props: { orgId: 'o', file: audio } })
  await flushPromises()

  const el = wrapper.get('audio')
  expect(el.attributes('src')).toBe('blob:audio')
  Object.defineProperty(el.element, 'duration', { value: 100 })
  await el.trigger('loadedmetadata')
  expect((el.element as HTMLAudioElement).currentTime).toBe(42)

  Object.defineProperty(el.element, 'currentTime', { value: 61.7, configurable: true })
  await el.trigger('timeupdate')
  expect(save).not.toHaveBeenCalled() // debounced

  await wrapper.get('button').trigger('click')
  expect(save).toHaveBeenCalledWith('o', 'f1', 61)
  expect(wrapper.emitted('close')).toEqual([[61]])
  wrapper.unmount()
  expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:audio')
})

it('shows a readable error when content is forbidden', async () => {
  vi.spyOn(files, 'fetchContent').mockRejectedValue(new ApiError(403, { code: 'forbidden', message: 'Kamu tidak punya akses untuk ini' }))
  const wrapper = mount(FileReader, { props: { orgId: 'o', file: audio } })
  await flushPromises()
  expect(wrapper.get('[role=alert]').text()).toContain('tidak punya akses')
  expect(wrapper.find('audio').exists()).toBe(false)
})
