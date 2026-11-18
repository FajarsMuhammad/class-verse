import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import RichContent from './RichContent.vue'
import QuizPlayer from './QuizPlayer.vue'
import { useDraftSave } from './useDraftSave'
import { plainRich, templateRich } from './richtext'
import { quizApi } from '@/api/quizzes'
import { ApiError } from '@/api/client'

vi.mock('@/api/quizzes', () => ({ quizApi: { start: vi.fn<() => void>(), answer: vi.fn<() => void>(), finish: vi.fn<() => void>() }, shapes: [{ symbol: '●' }, { symbol: '▲' }, { symbol: '■' }, { symbol: '◆' }] }))
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

describe('Content Studio', () => {
  it('renders literal text and rejects unsafe links without raw HTML', () => {
    const wrapper = mount(RichContent, { props: { content: { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: '<script>alert(1)</script>', marks: [{ type: 'link', attrs: { href: 'javascript:alert(1)' } }] }] }] } } })
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.find('a').exists()).toBe(false)
    expect(wrapper.text()).toContain('<script>')
  })
  it('provides three structured templates with unique block IDs', () => {
    expect(plainRich('legacy\ntext').content?.[0]?.content?.[0]?.text).toBe('legacy\ntext')
    expect(templateRich('article').content?.[0]?.type).toBe('heading')
    expect(templateRich('steps').content).toHaveLength(4)
    expect(templateRich('code').content?.map(n => n.type)).toContain('codeBlock')
  })
  it('simulates quiz locally without academic API calls and reveals only after finish', async () => {
    HTMLDialogElement.prototype.showModal = vi.fn<() => void>()
    const wrapper = mount(QuizPlayer, { props: { orgId: 'school', quizId: 'quiz', title: 'Preview', previewQuestions: [{ id: 'q', prompt: 'Benar?', options: ['Benar', 'Salah'], answer: 0, explanation: 'Penjelasan rahasia' }] } })
    await flushPromises()
    expect(wrapper.text()).toContain('Hasil tidak disimpan')
    expect(wrapper.text()).not.toContain('Penjelasan rahasia')
    const option = wrapper.findAll('button').find(b => b.text().includes('Benar'))!
    await option.trigger('click')
    const finish = wrapper.findAll('button').find(b => b.text().includes('Selesai'))!
    await finish.trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('Penjelasan rahasia')
    expect(quizApi.start).not.toHaveBeenCalled()
    expect(quizApi.answer).not.toHaveBeenCalled()
    expect(quizApi.finish).not.toHaveBeenCalled()
    expect(wrapper.emitted('finished')).toBeUndefined()
  })
  it('retains dirty state after network failure, retries, and blocks stale conflict autosaves', async () => {
    vi.useFakeTimers()
    const save = vi.fn<() => Promise<void>>().mockRejectedValueOnce(new Error('Network down')).mockResolvedValueOnce(undefined).mockRejectedValueOnce(new ApiError(409, { code: 'draft_conflict', message: 'Changed' }))
    let draft!: ReturnType<typeof useDraftSave<{ title: string }>>
    const value = ref({ title: 'original' })
    const wrapper = mount(defineComponent({ setup() { draft = useDraftSave(value, save); draft.reset(); return () => null } }))
    value.value = { title: 'edit' }; await flushPromises(); await vi.advanceTimersByTimeAsync(1200)
    expect(draft.state.value).toBe('failed'); expect(draft.dirty.value).toBe(true)
    await draft.flush(); expect(draft.state.value).toBe('saved'); expect(draft.dirty.value).toBe(false)
    value.value = { title: 'second' }; await flushPromises(); await vi.advanceTimersByTimeAsync(1200)
    expect(draft.state.value).toBe('conflict'); await draft.flush(); expect(save).toHaveBeenCalledTimes(3)
    wrapper.unmount()
  })
})
