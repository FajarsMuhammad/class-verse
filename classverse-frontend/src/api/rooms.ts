import { api } from './client'
import type { CreateRoomRequest, Room, RoomList, UpdateRoomRequest } from './types'

const enc = encodeURIComponent

export const roomApi = {
  list: (orgId: string, options: { building?: string; cursor?: string } = {}) => {
    const query = new URLSearchParams()
    if (options.building) query.set('building', options.building)
    if (options.cursor) query.set('cursor', options.cursor)
    return api<RoomList>(`/api/orgs/${enc(orgId)}/rooms${query.size ? `?${query}` : ''}`)
  },
  get: (id: string) => api<Room>(`/api/rooms/${enc(id)}`),
  create: (orgId: string, input: CreateRoomRequest) =>
    api<Room>(`/api/orgs/${enc(orgId)}/rooms`, { method: 'POST', body: input }),
  update: (id: string, input: UpdateRoomRequest) =>
    api<Room>(`/api/rooms/${enc(id)}`, { method: 'PATCH', body: input }),
  archive: (id: string) => api<void>(`/api/rooms/${enc(id)}`, { method: 'DELETE' }),
}
