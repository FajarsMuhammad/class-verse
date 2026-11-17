import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import InvitePanel from './InvitePanel.vue'
import { inviteApi } from '@/api/orgs'

vi.mock('@/api/orgs', () => ({ inviteApi: { create: vi.fn<typeof inviteApi.create>(), list: vi.fn<typeof inviteApi.list>(), revoke: vi.fn<typeof inviteApi.revoke>() } }))
beforeEach(() => {
  vi.mocked(inviteApi.list).mockResolvedValue({ items: [], next_cursor: null })
  vi.mocked(inviteApi.create).mockResolvedValue({ id: 'i', code: 'ABCD-EFGH', link: '/gabung/ABCD-EFGH', role: 'participant', class_id: 'c', max_uses: null, used_count: 0, created_by: 'u', expires_at: '2026-11-23T00:00:00Z', created_at: '2026-11-17T00:00:00Z' })
})
it('creates a participant-only invitation to the selected class', async () => {
  const w = mount(InvitePanel, { props: { orgId: 'o', myRole: 'teacher', myId: 'u', classId: 'c' } })
  await flushPromises()
  expect(w.text()).toContain('Undang siswa ke kelas ini')
  expect(w.text()).not.toContain('Sebagai')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(inviteApi.create).toHaveBeenCalledWith('o', { class_id: 'c', role: 'participant', expires_in_days: 7, max_uses: null })
  expect(w.text()).toContain('ABCD-EFGH')
})
