import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import FileUploadPanel from './FileUploadPanel.vue'
import { ApiError } from '@/api/client'
import * as files from '@/api/files'
import * as uploads from '@/api/uploads'
import type { FileItem } from '@/api/types'

const item = (id: string, owner: string, position = 0): FileItem => ({
  id,
  name: `${id}.pdf`,
  kind: 'pdf',
  content_type: 'application/pdf',
  size: 2_500_000,
  uploaded_by: owner,
  created_at: '2026-01-01T00:00:00Z',
  position,
})

afterEach(() => vi.restoreAllMocks())

function setup(isAdmin = false) {
  const list = vi.spyOn(files.fileApi, 'list').mockResolvedValue({
    items: [item('mine', 'me', 3), item('theirs', 'other')],
    used_bytes: 5_000_000,
    quota_bytes: 5_000_000_000,
  })
  const wrapper = mount(FileUploadPanel, { props: { orgId: 'o', userId: 'me', isAdmin }, global: { stubs: { FileReader: true } } })
  return { wrapper, list }
}

it('lists files with usage, resume label, and delete only for own files', async () => {
  const { wrapper } = setup()
  await flushPromises()
  expect(wrapper.text()).toContain('Terpakai 5 MB dari 5.000 MB')
  const rows = wrapper.findAll('li')
  expect(rows[0]!.text()).toContain('Lanjutkan')
  expect(rows[0]!.text()).toContain('Hapus')
  expect(rows[1]!.text()).toContain('Baca')
  expect(rows[1]!.text()).not.toContain('Hapus')
})

it('admin can delete any file after confirmation', async () => {
  const { wrapper, list } = setup(true)
  await flushPromises()
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  const remove = vi.spyOn(files.fileApi, 'remove').mockResolvedValue()
  await wrapper.findAll('li')[1]!.findAll('button')[1]!.trigger('click')
  await flushPromises()
  expect(remove).toHaveBeenCalledWith('o', 'theirs')
  expect(list).toHaveBeenCalledTimes(2)
})

it('does not suggest resuming when the server rejected the file format', async () => {
  const { wrapper } = setup()
  await flushPromises()
  vi.spyOn(uploads, 'uploadFile').mockRejectedValue(new ApiError(400, { code: 'validation_failed', message: 'x' }))
  const input = wrapper.get('input[type=file]')
  Object.defineProperty(input.element, 'files', { value: [new File(['x'], 'buku.pdf', { type: 'application/pdf' })] })
  await input.trigger('change')
  await wrapper.get('button.btn-primary').trigger('click')
  await flushPromises()
  const alert = wrapper.get('[role=alert]').text()
  expect(alert).toContain('bukan PDF')
  expect(alert).not.toContain('lanjutkan')
})

it('rejects unsupported types before uploading', async () => {
  const { wrapper } = setup()
  await flushPromises()
  const upload = vi.spyOn(uploads, 'uploadFile')
  const input = wrapper.get('input[type=file]')
  Object.defineProperty(input.element, 'files', { value: [new File(['x'], 'video.mp4', { type: 'video/mp4' })] })
  await input.trigger('change')
  await wrapper.get('button.btn-primary').trigger('click')
  expect(upload).not.toHaveBeenCalled()
  expect(wrapper.get('[role=alert]').text()).toContain('PDF, MP3, atau M4A')
})
