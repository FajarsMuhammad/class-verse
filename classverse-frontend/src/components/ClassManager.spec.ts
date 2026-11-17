import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ClassManager from './ClassManager.vue'
import { classApi } from '@/api/classes'

vi.mock('@/api/classes', () => ({ classApi: {
  years: vi.fn<typeof classApi.years>(), classes: vi.fn<typeof classApi.classes>(), roster: vi.fn<typeof classApi.roster>(),
  createYear: vi.fn<typeof classApi.createYear>(), activate: vi.fn<typeof classApi.activate>(),
  create: vi.fn<typeof classApi.create>(), rename: vi.fn<typeof classApi.rename>(), archive: vi.fn<typeof classApi.archive>(),
  place: vi.fn<typeof classApi.place>(), history: vi.fn<typeof classApi.history>(),
} }))
const years = [{ id: 'y1', label: '2026/2027', active: true }, { id: 'y2', label: '2027/2028', active: false }]
const classes = [{ id: 'c1', academic_year_id: 'y1', name: 'XI IPA 1', students: 1 }]
const roster = [{ user_id: 'u1', display_name: 'Ani', avatar_key: 'default', class_id: 'c1' },
  { user_id: 'u2', display_name: 'Budi', avatar_key: 'default', class_id: null }]
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(classApi.years).mockResolvedValue({ items: years })
  vi.mocked(classApi.classes).mockResolvedValue({ items: classes })
  vi.mocked(classApi.roster).mockResolvedValue({ items: roster })
  vi.mocked(classApi.history).mockResolvedValue({ items: [] })
})
const make = (isAdmin = false) => mount(ClassManager, { props: { orgId: 'org', isAdmin } })

describe('ClassManager', () => {
  it('loads active year, classes and unplaced students; hides admin-only controls', async () => {
    const w = make()
    await flushPromises()
    expect(classApi.roster).toHaveBeenCalledWith('org', 'y1')
    expect(w.text()).toContain('Belum ditempatkan')
    expect(w.text()).toContain('Ani')
    expect(w.find('[data-test="create-year"]').exists()).toBe(false)
    expect(w.find('[data-test="activate-year"]').exists()).toBe(false)
  })
  it('moves a student only after confirmation then reloads server truth', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const w = make()
    await flushPromises()
    await w.get('[data-student="u1"] select').setValue('')
    await flushPromises()
    expect(classApi.place).not.toHaveBeenCalled()
    confirm.mockReturnValue(true)
    await w.get('[data-student="u2"] select').setValue('c1')
    await flushPromises()
    expect(classApi.place).toHaveBeenCalledWith('org', 'y1', 'u2', 'c1')
    expect(classApi.roster).toHaveBeenCalledTimes(2)
    confirm.mockRestore()
  })
  it('keeps previous placement and shows error when saving fails', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.mocked(classApi.place).mockRejectedValue(new Error('Simpan gagal'))
    const w = make()
    await flushPromises()
    await w.get('[data-student="u1"] select').setValue('')
    await flushPromises()
    expect((w.get('[data-student="u1"] select').element as HTMLSelectElement).value).toBe('c1')
    expect(w.get('[role="alert"]').text()).toContain('Simpan gagal')
    vi.restoreAllMocks()
  })
  it('lets admin create and choose year without moving anyone automatically', async () => {
    const w = make(true)
    await flushPromises()
    await w.get('[data-test="year"]').setValue('y2')
    await flushPromises()
    expect(classApi.roster).toHaveBeenLastCalledWith('org', 'y2')
    expect(w.find('[data-test="activate-year"]').exists()).toBe(true)
    expect(classApi.place).not.toHaveBeenCalled()
    await w.get('[data-test="activate-year"]').trigger('click')
    await flushPromises()
    expect(classApi.activate).toHaveBeenCalledWith('org', 'y2')
  })
  it('shows placement history on demand', async () => {
    vi.mocked(classApi.history).mockResolvedValue({ items: [{ class_name: 'X IPA', year_label: '2025/2026', started_at: '2025-07-01T00:00:00Z', ended_at: '2026-07-01T00:00:00Z' }] })
    const w = make()
    await flushPromises()
    await w.get('[data-student="u1"] button').trigger('click')
    await flushPromises()
    expect(classApi.history).toHaveBeenCalledWith('org', 'u1')
    expect(w.text()).toContain('X IPA · 2025/2026')
  })
  it('handles empty school and failed initial load with retry', async () => {
    vi.mocked(classApi.years).mockRejectedValueOnce(new Error('Jaringan gagal')).mockResolvedValue({ items: [] })
    const w = make(true)
    await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('Jaringan gagal')
    await w.get('[data-test="retry"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Belum ada tahun ajaran')
    expect(classApi.roster).not.toHaveBeenCalled()
    expect(w.find('[data-test="create-year"]').exists()).toBe(true)
  })
})
