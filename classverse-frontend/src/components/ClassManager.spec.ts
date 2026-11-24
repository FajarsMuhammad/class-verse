import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ClassManager from './ClassManager.vue'
import ClassTeachers from './ClassTeachers.vue'
import YearSetup from './YearSetup.vue'
import { classApi, type SchoolClass } from '@/api/classes'

vi.mock('@/api/classes', () => ({ classApi: {
  years: vi.fn<typeof classApi.years>(), classes: vi.fn<typeof classApi.classes>(), roster: vi.fn<typeof classApi.roster>(),
  createYear: vi.fn<typeof classApi.createYear>(), activate: vi.fn<typeof classApi.activate>(),
  create: vi.fn<typeof classApi.create>(), rename: vi.fn<typeof classApi.rename>(), archive: vi.fn<typeof classApi.archive>(),
  place: vi.fn<typeof classApi.place>(), history: vi.fn<typeof classApi.history>(), move: vi.fn<typeof classApi.move>(),
  copy: vi.fn<typeof classApi.copy>(), promote: vi.fn<typeof classApi.promote>(), staff: vi.fn<typeof classApi.staff>(),
  teachers: vi.fn<typeof classApi.teachers>(), setTeacher: vi.fn<typeof classApi.setTeacher>(), removeTeacher: vi.fn<typeof classApi.removeTeacher>(),
} }))
const years = [{ id: 'y1', label: '2026/2027', active: true }, { id: 'y2', label: '2027/2028', active: false }]
const cls = (id: string, name: string, extra: Partial<SchoolClass> = {}): SchoolClass =>
  ({ id, academic_year_id: 'y1', name, students: 1, subject_teachers: 0, homeroom: null, my_role: null, ...extra })
const classes = [cls('c1', 'XI IPA 1', { my_role: 'homeroom', homeroom: { id: 't1', name: 'Bu Sari' } }), cls('c2', 'XI IPA 2')]
const ani = { user_id: 'u1', display_name: 'Ani', avatar_key: 'default', class_id: 'c1' }
const budi = { user_id: 'u2', display_name: 'Budi', avatar_key: 'default', class_id: null }
beforeEach(() => {
  vi.resetAllMocks()
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  vi.mocked(classApi.years).mockResolvedValue({ items: years })
  vi.mocked(classApi.classes).mockResolvedValue({ items: classes })
  vi.mocked(classApi.roster).mockImplementation(async (_o, _y, f = {}) => {
    const all = [ani, budi].filter(r => !f.class_id || (f.class_id === 'none' ? !r.class_id : r.class_id === f.class_id))
    return { items: all.slice(0, f.limit ?? 50), total: all.length }
  })
  vi.mocked(classApi.history).mockResolvedValue({ items: [] })
  vi.mocked(classApi.teachers).mockResolvedValue({ items: [{ user_id: 't1', display_name: 'Bu Sari', role: 'homeroom', subject: null }] })
  vi.mocked(classApi.staff).mockResolvedValue({ items: [{ user_id: 't1', display_name: 'Bu Sari', role: 'teacher' }, { user_id: 't2', display_name: 'Pak Dodi', role: 'teacher' }] })
})
const make = (isAdmin = false, view: 'classes' | 'students' | 'years' = 'students') => mount(ClassManager, { props: { orgId: 'org', isAdmin, view } })

describe('StudentRoster (Kelola → Siswa)', () => {
  it('asks the server for one filtered page and shows the unplaced count', async () => {
    const w = make(true)
    await flushPromises()
    expect(classApi.roster).toHaveBeenCalledWith('org', 'y1', { class_id: '', q: '', offset: 0, limit: 50 })
    expect(w.text()).toContain('Belum punya kelas (1)')
    expect(w.text()).toContain('1–2 dari 2 siswa')
    expect(w.find('table[aria-label="Daftar siswa"]').exists()).toBe(true)
    await w.get('[data-filter="none"]').trigger('click')
    await flushPromises()
    expect(classApi.roster).toHaveBeenCalledWith('org', 'y1', { class_id: 'none', q: '', offset: 0, limit: 50 })
    expect(w.find('[data-student="u1"]').exists()).toBe(false)
  })
  it('admin moves selected students in bulk and reports failures per student', async () => {
    vi.mocked(classApi.move).mockResolvedValue({ items: [{ user_id: 'u1', error: null }, { user_id: 'u2', error: 'not_student' }] })
    const w = make(true)
    await flushPromises()
    await w.get('[data-test="select-page"]').setValue(true)
    await flushPromises()
    await w.get('[data-test="move-target"]').setValue('c2')
    await w.get('[data-action="move"]').trigger('click')
    await flushPromises()
    expect(classApi.move).toHaveBeenCalledWith('org', 'y1', ['u1', 'u2'], 'c2')
    expect(w.get('[data-test="notice"]').text()).toBe('1 siswa dipindahkan. Tidak dipindahkan: Budi (bukan peserta sekolah).')
  })
  it('non-admin without a homeroom only reads: no select, no bulk', async () => {
    vi.mocked(classApi.classes).mockResolvedValue({ items: [cls('c2', 'XI IPA 2', { my_role: 'subject' })] })
    const w = make(false)
    await flushPromises()
    expect(w.find('[data-student="u1"] select').exists()).toBe(false)
    expect(w.find('[data-test="select-page"]').exists()).toBe(false)
  })
  it('wali kelas can bulk move only into own class', async () => {
    const w = make(false)
    await flushPromises()
    await w.get('[data-test="select-page"]').setValue(true)
    await flushPromises()
    const options = w.findAll('[data-test="move-target"] option').map(o => o.text())
    expect(options).toEqual(['Pilih kelas', 'XI IPA 1'])
  })
  it('shows class chips instead of individual move selects and paginates', async () => {
    vi.mocked(classApi.roster).mockResolvedValue({ items: [ani], total: 120 })
    const w = make(true)
    await flushPromises()
    expect(w.find('[data-student="u1"] select').exists()).toBe(false)
    expect(w.get('[data-student="u1"] .badge').text()).toBe('XI IPA 1')
    await w.get('[data-action="next"]').trigger('click')
    await flushPromises()
    expect(classApi.roster).toHaveBeenCalledWith('org', 'y1', expect.objectContaining({ offset: 50 }))
  })
})

describe('ClassManager classes', () => {
  it('shows wali kelas, guru mapel count and own role; only admin creates', async () => {
    const w = make(false, 'classes')
    await flushPromises()
    expect(w.get('[data-class="c1"]').text()).toContain('Wali kelas: Bu Sari · 0 guru mapel')
    expect(w.get('[data-class="c1"]').text()).toContain('Wali kelas Anda')
    expect(w.find('.new').exists()).toBe(false)
    await w.get('[data-class="c1"] button').trigger('click')
    expect(w.find('[data-tab="settings"]').exists()).toBe(false)
  })
  it('wali kelas adds a student without a class and removes a member in the detail', async () => {
    const w = make(false, 'classes')
    await flushPromises()
    await w.get('[data-class="c1"] button').trigger('click')
    await flushPromises()
    expect(w.get('.detail-title').text()).toBe('XI IPA 1')
    expect(w.find('aside[aria-label="Detail kelas"]').exists()).toBe(true)
    expect(w.find('.class-catalog [data-class="c2"]').exists()).toBe(true)
    expect(w.find('[data-student="u1"]').exists()).toBe(true)
    await w.get('[data-test="add-member"] select').setValue('u2')
    await w.get('[data-test="add-member"]').trigger('submit')
    await flushPromises()
    expect(classApi.place).toHaveBeenCalledWith('org', 'y1', 'u2', 'c1')
    await w.get('[data-student="u1"] button').trigger('click')
    await flushPromises()
    expect(classApi.place).toHaveBeenLastCalledWith('org', 'y1', 'u1', null)
  })
  it('admin renames, creates and sees archive blocked while students remain', async () => {
    const created = cls('c3', 'X IPS 1', { students: 0 })
    vi.mocked(classApi.rename).mockResolvedValue(classes[0]!)
    vi.mocked(classApi.create).mockResolvedValue(created)
    const w = make(true, 'classes')
    await flushPromises()
    await w.get('[data-class="c1"] button').trigger('click')
    await w.get('[data-tab="settings"]').trigger('click')
    expect(w.get('[data-action="archive"]').attributes('disabled')).toBeDefined()
    await w.get('.settings input').setValue('XI IPA 3')
    await w.get('.settings form').trigger('submit')
    await flushPromises()
    expect(classApi.rename).toHaveBeenCalledWith('org', 'c1', 'XI IPA 3')
    await w.get('[data-action="back"]').trigger('click')
    vi.mocked(classApi.classes).mockResolvedValue({ items: [...classes, created] })
    await w.get('.new').trigger('click')
    await w.get('.create input').setValue('X IPS 1')
    await w.get('.create').trigger('submit')
    await flushPromises()
    expect(w.get('.detail-title').text()).toBe('X IPS 1')
  })
})

describe('ClassTeachers', () => {
  it('admin sets a guru mapel with subject and replaces wali kelas after confirmation', async () => {
    const w = mount(ClassTeachers, { props: { orgId: 'org', classId: 'c1', isAdmin: true } })
    await flushPromises()
    expect(w.get('[data-teacher="homeroom"]').text()).toContain('Bu Sari')
    await w.get('[data-test="add-teacher"] select').setValue('t2')
    await w.get('[data-test="add-teacher"] input').setValue('Fisika')
    await w.get('[data-test="add-teacher"]').trigger('submit')
    await flushPromises()
    expect(classApi.setTeacher).toHaveBeenCalledWith('org', 'c1', 't2', 'subject', 'Fisika')
    await w.get('[data-test="add-teacher"] select').setValue('t2')
    await w.get('[data-test="teacher-role"]').setValue('homeroom')
    await w.get('[data-test="add-teacher"]').trigger('submit')
    await flushPromises()
    expect(window.confirm).toHaveBeenCalledWith('Ganti wali kelas Bu Sari dengan Pak Dodi?')
    expect(classApi.setTeacher).toHaveBeenLastCalledWith('org', 'c1', 't2', 'homeroom', '')
    expect(w.emitted('changed')).toHaveLength(2)
  })
  it('others read only and never load staff', async () => {
    const w = mount(ClassTeachers, { props: { orgId: 'org', classId: 'c1', isAdmin: false } })
    await flushPromises()
    expect(classApi.staff).not.toHaveBeenCalled()
    expect(w.find('[data-test="add-teacher"]').exists()).toBe(false)
    expect(w.find('[data-teacher="homeroom"] button').exists()).toBe(false)
  })
})

describe('YearSetup', () => {
  it('copies structure then promotes with explicit mapping after preview', async () => {
    vi.mocked(classApi.classes).mockImplementation(async (_o, y) => ({ items: y === 'y1' ? classes : [cls('n1', 'XII IPA 1', { academic_year_id: 'y2', students: 0 })] }))
    vi.mocked(classApi.copy).mockResolvedValue({ created: 1, skipped: ['XI IPA 2'] })
    vi.mocked(classApi.promote).mockResolvedValue({ moved: 1, skipped: [{ id: 'u9', name: 'Cici' }] })
    const w = mount(YearSetup, { props: { orgId: 'org', years: [years[1]!, years[0]!] } })
    await flushPromises()
    await w.get('[data-test="from-year"]').setValue('y1')
    await flushPromises()
    await w.get('[data-action="copy"]').trigger('click')
    await flushPromises()
    expect(classApi.copy).toHaveBeenCalledWith('org', 'y2', 'y1', true)
    expect(w.get('[data-test="setup-notice"]').text()).toBe('1 kelas dibuat. Sudah ada, dilewati: XI IPA 2.')
    expect(w.get('[data-action="preview"]').attributes('disabled')).toBeDefined()
    await w.get('[data-from="c1"] select').setValue('n1')
    await w.get('[data-action="preview"]').trigger('click')
    expect(w.get('[data-test="preview"]').text()).toContain('XI IPA 1 (1 siswa) → XII IPA 1')
    expect(classApi.promote).not.toHaveBeenCalled()
    await w.get('[data-action="promote"]').trigger('click')
    await flushPromises()
    expect(classApi.promote).toHaveBeenCalledWith('org', 'y2', 'y1', [{ from_class_id: 'c1', to_class_id: 'n1' }])
    expect(w.text()).toContain('Cici')
  })
})

describe('ClassManager years', () => {
  it('admin adds and activates a year after confirmation, without loading roster', async () => {
    const w = make(true, 'years')
    await flushPromises()
    expect(classApi.roster).not.toHaveBeenCalled()
    expect(w.find('[data-test="year-setup"]').exists()).toBe(true)
    await w.get('[data-year="y2"] button').trigger('click')
    await flushPromises()
    expect(classApi.activate).toHaveBeenCalledWith('org', 'y2')
  })
  it('admin creates a new year from the separate form and sees year counts', async () => {
    vi.mocked(classApi.years).mockResolvedValue({ items: [{ ...years[0]!, class_count: 4, student_count: 90 }], total: 1 })
    const w = make(true, 'years')
    await flushPromises()
    expect(w.get('[data-year="y1"]').text()).toContain('4 kelas · 90 siswa')
    await w.get('[data-test="create-year"] input').setValue('2028/2029')
    await w.get('[data-test="create-year"]').trigger('submit')
    await flushPromises()
    expect(classApi.createYear).toHaveBeenCalledWith('org', '2028/2029')
  })
  it('handles failed initial load with retry and points non-admin to admin', async () => {
    vi.mocked(classApi.years).mockRejectedValueOnce(new Error('Jaringan gagal')).mockResolvedValue({ items: [] })
    const w = make(true, 'years')
    await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('Jaringan gagal')
    await w.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Belum ada tahun ajaran')
    const s = make(false, 'classes')
    await flushPromises()
    expect(s.text()).toContain('Minta admin sekolah')
  })
})
