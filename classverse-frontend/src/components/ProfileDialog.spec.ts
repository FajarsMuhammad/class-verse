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
    expect(wrapper.findAll('input[type=radio]')).toHaveLength(21)
    await wrapper.get('#profile-name').setValue('Ani Baru')
    await wrapper.get('input[name="avatar-skin"][value="5"]').setValue()
    await wrapper.get('input[name="avatar-hair"][value="4"]').setValue()
    await wrapper.get('input[name="avatar-shirt"][value="9"]').setValue()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(save).toHaveBeenCalledWith({ display_name: 'Ani Baru', avatar_key: 'look-5-4-9' })
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
  it('loads saved sporty colors and changes clothing without changing skin or hair', async () => {
    const { wrapper, save } = setup('sporty-headphones-koral')
    expect((wrapper.get('input[name="avatar-skin"][value="2"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('input[name="avatar-shirt"][value="1"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('input[name="avatar-shirt"][value="3"]').setValue()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(save).toHaveBeenCalledWith({ avatar_key: 'look-2-2-3' })
  })
  it('color selection on a legacy avatar preserves its skin and hair', async () => {
    const { wrapper, save } = setup()
    await wrapper.get('input[name="avatar-shirt"][value="8"]').setValue()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(save).toHaveBeenCalledWith({ avatar_key: 'look-0-0-8' })
  })
  it('cancel does not save', async () => {
    const { wrapper, save } = setup()
    await wrapper.get('#profile-name').setValue('Draft')
    const cancel = wrapper.findAll('button').find((b) => b.text() === 'Batal')!
    await cancel.trigger('click')
    expect(save).not.toHaveBeenCalled()
  })
})
