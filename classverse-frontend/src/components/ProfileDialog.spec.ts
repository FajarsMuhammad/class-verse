import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfileDialog from './ProfileDialog.vue'
import { useAuthStore } from '@/stores/auth'
import { ApiError } from '@/api/client'

beforeEach(() => {
  setActivePinia(createPinia())
  HTMLDialogElement.prototype.showModal = vi.fn<() => void>()
  HTMLDialogElement.prototype.close = vi.fn<() => void>()
})
function setup(avatarKey = 'default') {
  const auth = useAuthStore()
  auth.me = {
    id: 'u',
    email: 'a@b.co',
    display_name: 'Ani',
    avatar_key: avatarKey,
    birth_date: '2000-01-01',
    needs_profile: false,
    orgs: [],
  }
  const save = vi.spyOn(auth, 'updateProfile').mockResolvedValue()
  const wrapper = mount(ProfileDialog)
  return { auth, save, wrapper }
}
describe('ProfileDialog', () => {
  it('submits selected preset and edited name only', async () => {
    const { wrapper, save } = setup()
    expect(wrapper.findAll('input[type=radio]')).toHaveLength(13)
    await wrapper.get('#profile-name').setValue('Ani Baru')
    await wrapper.get('input[name="avatar-style"][value="glasses"]').setValue()
    await wrapper.get('input[name="avatar-color"][value="zamrud"]').setValue()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(save).toHaveBeenCalledWith({ display_name: 'Ani Baru', avatar_key: 'sporty-glasses-zamrud' })
    expect(wrapper.emitted('saved')).toHaveLength(1)
  })
  it('reports field error and keeps stored state on failure', async () => {
    const { wrapper, save, auth } = setup()
    save.mockRejectedValue(
      new ApiError(400, {
        code: 'validation_failed',
        message: 'Periksa profil',
        fields: { display_name: 'Nama ditolak' },
      }),
    )
    await wrapper.get('#profile-name').setValue('Invalid')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('#name-error').text()).toBe('Nama ditolak')
    expect(auth.me?.display_name).toBe('Ani')
    expect(wrapper.emitted('saved')).toBeUndefined()
  })
  it('does not replace a legacy avatar when only the name changes', async () => {
    const { wrapper, save } = setup()
    await wrapper.get('#profile-name').setValue('Ani Baru')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(save).toHaveBeenCalledWith({ display_name: 'Ani Baru' })
  })
  it('loads saved sporty choices and allows changing color only', async () => {
    const { wrapper, save } = setup('sporty-headphones-koral')
    expect((wrapper.get('input[value="headphones"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('input[value="koral"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('input[value="laut"]').setValue()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(save).toHaveBeenCalledWith({ avatar_key: 'sporty-headphones-laut' })
  })
  it('color selection on a legacy avatar selects the default sporty style', async () => {
    const { wrapper, save } = setup()
    await wrapper.get('input[value="grafit"]').setValue()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(save).toHaveBeenCalledWith({ avatar_key: 'sporty-runner-grafit' })
  })
  it('cancel does not save', async () => {
    const { wrapper, save } = setup()
    await wrapper.get('#profile-name').setValue('Draft')
    const cancel = wrapper.findAll('button').find((b) => b.text() === 'Batal')!
    await cancel.trigger('click')
    expect(save).not.toHaveBeenCalled()
  })
})
