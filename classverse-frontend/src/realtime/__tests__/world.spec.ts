import { describe, expect, it } from 'vitest'

import type { ServerMessage } from '../messages'
import { RENDER_DELAY_MS, World } from '../world'

const p = (id: string, x: number, y = 0) => ({
  id,
  name: id,
  avatar: 'default',
  role: 'participant',
  x,
  y,
  dir: 'down' as const,
})

function welcomed() {
  const w = new World()
  w.apply(
    {
      t: 'welcome',
      seq: 1,
      d: {
        self_id: 'me',
        room_id: 'r',
        map_key: 'campus',
        participants: [p('me', 5), p('b', 0)],
        all_chat_enabled: true,
      },
    },
    0,
  )
  return w
}

describe('World', () => {
  it('replaces origin presence and self position only after traveled', () => {
    const w = welcomed()
    w.apply({ t: 'error', seq: 2, d: { code: 'room_full', message: 'Penuh' } }, 10)
    expect(w.mapKey).toBe('campus')
    expect(w.avatars.has('b')).toBe(true)
    w.apply(
      {
        t: 'traveled',
        seq: 3,
        d: {
          self_id: 'me',
          room_id: 'class-1',
          map_key: 'classroom',
          room_name: 'Biologi',
          building_key: 'kelas-a',
          participants: [p('me', 480, 520), p('teacher', 100)],
          all_chat_enabled: true,
        },
      },
      20,
    )
    expect(w.mapKey).toBe('classroom')
    expect(w.self).toEqual({ x: 480, y: 520, dir: 'down' })
    expect([...w.avatars.keys()]).toEqual(['teacher'])
    expect(w.count).toBe(2)
  })
  it('updates self and remote identity without moving or unhiding avatars', () => {
    const w = welcomed()
    const before = { ...w.self! }
    w.apply(
      { t: 'profile_updated', seq: 2, d: { id: 'me', name: 'Ani Baru', avatar: 'bun-nila' } },
      100,
    )
    w.apply({ t: 'snapshot', seq: 3, d: { p: [], out: ['b'] } }, 100)
    w.apply(
      { t: 'profile_updated', seq: 4, d: { id: 'b', name: 'Budi Baru', avatar: 'cap-mangga' } },
      100,
    )
    expect(w.selfName).toBe('Ani Baru')
    expect(w.selfAvatar).toBe('bun-nila')
    expect(w.self).toEqual(before)
    expect(w.avatars.get('b')).toMatchObject({
      name: 'Budi Baru',
      avatar: 'cap-mangga',
      hidden: true,
    })
  })
  it('separates self from others', () => {
    const w = welcomed()
    expect(w.self).toEqual({ x: 5, y: 0, dir: 'down' })
    expect([...w.avatars.keys()]).toEqual(['b'])
    expect(w.count).toBe(2)
  })

  it('interpolates between snapshots with a render delay', () => {
    const w = welcomed()
    w.apply(
      {
        t: 'snapshot',
        seq: 2,
        d: { p: [{ id: 'b', x: 100, y: 0, dir: 'right' }] },
      } as ServerMessage,
      100,
    )
    w.sample(50 + RENDER_DELAY_MS)
    expect(w.avatars.get('b')!.x).toBeCloseTo(50)
    w.sample(200 + RENDER_DELAY_MS)
    expect(w.avatars.get('b')!.x).toBe(100)
  })

  it('hides avatars that leave the area of interest and snaps them back', () => {
    const w = welcomed()
    w.apply({ t: 'snapshot', seq: 2, d: { p: [], out: ['b'] } }, 100)
    expect(w.avatars.get('b')!.hidden).toBe(true)
    w.apply({ t: 'snapshot', seq: 3, d: { p: [{ id: 'b', x: 900, y: 0, dir: 'left' }] } }, 200)
    const b = w.avatars.get('b')!
    expect(b.hidden).toBe(false)
    expect(b.x).toBe(900)
  })

  it('handles joined, left, and rejected moves', () => {
    const w = welcomed()
    w.apply({ t: 'joined', seq: 2, d: p('c', 1) }, 0)
    w.apply({ t: 'joined', seq: 3, d: p('me', 1) }, 0)
    w.apply({ t: 'left', seq: 4, d: { id: 'b' } }, 0)
    expect([...w.avatars.keys()]).toEqual(['c'])
    w.self!.x = 400
    w.apply({ t: 'error', seq: 5, d: { code: 'move_rejected', message: '', x: 7, y: 8 } }, 0)
    expect(w.self).toMatchObject({ x: 7, y: 8 })
  })
})
