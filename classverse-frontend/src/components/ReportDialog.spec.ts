import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ReportDialog from './ReportDialog.vue'
import { reportApi } from '@/api/reports'

beforeEach(() => {
  HTMLDialogElement.prototype.showModal = vi.fn<() => void>()
  HTMLDialogElement.prototype.close = vi.fn<() => void>()
  vi.restoreAllMocks()
})

const props = { targetId: 'budi', targetName: 'Budi', roomId: 'room-1' }

describe('ReportDialog', () => {
  it('reports a stored message by id only, never the client copy of its text', async () => {
    const create = vi.spyOn(reportApi, 'create').mockResolvedValue({} as never)
    const w = mount(ReportDialog, { props: { ...props, messageId: 42, messageText: 'kata ****' } })
    expect(w.text()).toContain('kata ****')
    await w.get('textarea').setValue('kasar')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(create).toHaveBeenCalledWith({ target_user_id: 'budi', message_id: 42, reason: 'kasar' })
    expect(w.emitted('reported')).toHaveLength(1)
  })

  it('falls back to a user report with the room for unsaved (campus) chat', async () => {
    const create = vi.spyOn(reportApi, 'create').mockResolvedValue({} as never)
    const w = mount(ReportDialog, { props })
    await w.get('textarea').setValue('mengganggu')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(create).toHaveBeenCalledWith({
      target_user_id: 'budi',
      room_id: 'room-1',
      reason: 'mengganggu',
    })
  })

  it('blocks empty and over-long reasons', async () => {
    const create = vi.spyOn(reportApi, 'create')
    const w = mount(ReportDialog, { props })
    await w.get('form').trigger('submit')
    await w.get('textarea').setValue('a'.repeat(1001))
    await w.get('form').trigger('submit')
    expect(create).not.toHaveBeenCalled()
    expect(w.get('button[type=submit]').attributes('disabled')).toBeDefined()
  })
})
