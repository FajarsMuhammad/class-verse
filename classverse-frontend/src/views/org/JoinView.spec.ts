import { flushPromises, mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import JoinView from './JoinView.vue'
import { inviteApi } from '@/api/orgs'

const mocks = vi.hoisted(() => ({ push: vi.fn<(route: unknown) => Promise<void>>(), reload: vi.fn<() => Promise<void>>(), select: vi.fn<(org: string) => void>() }))
vi.mock('vue-router', () => ({ useRoute: () => ({ params: { code: 'ABCD-EFGH' } }), useRouter: () => ({ push: mocks.push }), RouterLink: { template: '<a><slot /></a>' } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ reload: mocks.reload }) }))
vi.mock('@/stores/org', () => ({ useOrgStore: () => ({ select: mocks.select, orgs: [] }) }))
vi.mock('@/api/orgs', () => ({ inviteApi: { preview: vi.fn<typeof inviteApi.preview>(), accept: vi.fn<typeof inviteApi.accept>() }, normalizeInviteCode: () => 'ABCD-EFGH' }))

it('shows class/year preview and keeps membership conflict visible rather than redirecting', async () => {
  vi.mocked(inviteApi.preview).mockResolvedValue({ org_name: 'Sekolah', role: 'participant', class_name: 'X IPA', year_label: '2026/2027' })
  vi.mocked(inviteApi.accept).mockResolvedValue({ org_id: 'o', user_id: 'u', role: 'participant', joined_at: '2026-11-17T00:00:00Z', placement_status: 'has_class' })
  const w = mount(JoinView)
  await flushPromises()
  expect(w.text()).toContain('Kelas X IPA, tahun ajaran 2026/2027')
  await w.get('button.btn-primary').trigger('click')
  await flushPromises()
  expect(mocks.select).toHaveBeenCalledWith('o')
  expect(mocks.reload).toHaveBeenCalled()
  expect(mocks.push).not.toHaveBeenCalled()
  expect(w.text()).toContain('Kelasmu tidak diubah')
  expect(w.text()).toContain('Buka sekolah')
})
