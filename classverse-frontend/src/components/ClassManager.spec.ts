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
const make = (isAdmin = false, view: 'classes' | 'students' | 'years' = 'students') => mount(ClassManager, { props: { orgId: 'org', isAdmin, view } })

describe('ClassManager students', () => {
  it('loads active year roster with class filter including students without a class', async () => {
    const w = make()
    await flushPromises()
    expect(classApi.roster).toHaveBeenCalledWith('org', 'y1')
    expect(w.text()).toContain('2 dari 2 siswa')
    await w.get('[data-test="class-filter"]').setValue('none')
    expect(w.find('[data-student="u1"]').exists()).toBe(false)
    expect(w.find('[data-student="u2"]').exists()).toBe(true)
    expect(w.find('[data-test="create-year"]').exists()).toBe(false)
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
  it('switches year without moving anyone and shows history on demand', async () => {
    vi.mocked(classApi.history).mockResolvedValue({ items: [{ class_name: 'X IPA', year_label: '2025/2026', started_at: '2025-07-01T00:00:00Z', ended_at: '2026-07-01T00:00:00Z' }] })
    const w = make()
    await flushPromises()
    await w.get('[data-student="u1"] button').trigger('click')
    await flushPromises()
    expect(classApi.history).toHaveBeenCalledWith('org', 'u1')
    expect(w.text()).toContain('X IPA, 2025/2026')
    await w.get('[data-test="year"]').setValue('y2')
    await flushPromises()
    expect(classApi.roster).toHaveBeenLastCalledWith('org', 'y2')
    expect(classApi.place).not.toHaveBeenCalled()
  })
})

describe('ClassManager classes', () => {
  it('opens one class, adds an unplaced student and removes a member', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const w = make(false, 'classes')
    await flushPromises()
    expect(w.find('[data-student]').exists()).toBe(false)
    await w.get('[data-class="c1"] button').trigger('click')
    expect(w.get('.detail-title').text()).toBe('XI IPA 1')
    expect(w.find('[data-member="u1"]').exists()).toBe(true)
    expect(w.find('[data-member="u2"]').exists()).toBe(false)
    await w.get('[data-test="add-member"] select').setValue('u2')
    await w.get('[data-test="add-member"]').trigger('submit')
    await flushPromises()
    expect(classApi.place).toHaveBeenCalledWith('org', 'y1', 'u2', 'c1')
    await w.get('[data-member="u1"] button').trigger('click')
    await flushPromises()
    expect(classApi.place).toHaveBeenLastCalledWith('org', 'y1', 'u1', null)
    vi.restoreAllMocks()
  })
  it('renames in settings and blocks archive while students remain', async () => {
    vi.mocked(classApi.rename).mockResolvedValue(classes[0]!)
    const w = make(false, 'classes')
    await flushPromises()
    await w.get('[data-class="c1"] button').trigger('click')
    await w.get('[data-tab="settings"]').trigger('click')
    expect(w.get('[data-action="archive"]').attributes('disabled')).toBeDefined()
    await w.get('.settings input').setValue('XI IPA 2')
    await w.get('.settings form').trigger('submit')
    await flushPromises()
    expect(classApi.rename).toHaveBeenCalledWith('org', 'c1', 'XI IPA 2')
  })
  it('creates a class and opens it', async () => {
    const created = { id: 'c2', academic_year_id: 'y1', name: 'X IPS 1', students: 0 }
    vi.mocked(classApi.create).mockResolvedValue(created)
    vi.mocked(classApi.classes).mockResolvedValue({ items: [...classes, created] })
    const w = make(false, 'classes')
    await flushPromises()
    await w.get('.new').trigger('click')
    await w.get('.create input').setValue('X IPS 1')
    await w.get('.create').trigger('submit')
    await flushPromises()
    expect(classApi.create).toHaveBeenCalledWith('org', 'y1', 'X IPS 1')
    expect(w.get('.detail-title').text()).toBe('X IPS 1')
  })
})

describe('ClassManager years', () => {
  it('lets admin add and activate a year after confirmation, without loading roster', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const w = make(true, 'years')
    await flushPromises()
    expect(classApi.roster).not.toHaveBeenCalled()
    expect(w.get('[data-year="y1"]').text()).toContain('Aktif')
    await w.get('[data-year="y2"] button').trigger('click')
    await flushPromises()
    expect(classApi.activate).toHaveBeenCalledWith('org', 'y2')
    vi.restoreAllMocks()
  })
  it('handles empty school and failed initial load with retry', async () => {
    vi.mocked(classApi.years).mockRejectedValueOnce(new Error('Jaringan gagal')).mockResolvedValue({ items: [] })
    const w = make(true, 'years')
    await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('Jaringan gagal')
    await w.get('[data-test="retry"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Belum ada tahun ajaran')
    expect(w.find('[data-test="create-year"]').exists()).toBe(true)
  })
  it('points non-admin staff to admin when no year exists', async () => {
    vi.mocked(classApi.years).mockResolvedValue({ items: [] })
    const w = make(false, 'classes')
    await flushPromises()
    expect(w.text()).toContain('Minta admin sekolah')
    expect(classApi.roster).not.toHaveBeenCalled()
  })
})
