import { api } from './client'
import type { Book, BookInput, BookList, Shelf } from './types'

const base = (orgID: string) => `/api/orgs/${encodeURIComponent(orgID)}/library/books`

export interface BookQuery {
  shelf?: Shelf | ''
  q?: string
  drafts?: boolean
  cursor?: string | null
}

export const libraryApi = {
  list: (orgID: string, query: BookQuery = {}) => {
    const p = new URLSearchParams()
    if (query.shelf) p.set('shelf', query.shelf)
    if (query.q?.trim()) p.set('q', query.q.trim())
    if (query.drafts) p.set('drafts', 'true')
    if (query.cursor) p.set('cursor', query.cursor)
    const qs = p.toString()
    return api<BookList>(qs ? `${base(orgID)}?${qs}` : base(orgID))
  },
  create: (orgID: string, body: BookInput) => api<Book>(base(orgID), { method: 'POST', body }),
  update: (orgID: string, id: string, body: BookInput) =>
    api<Book>(`${base(orgID)}/${encodeURIComponent(id)}`, { method: 'PUT', body }),
  archive: (orgID: string, id: string) =>
    api<void>(`${base(orgID)}/${encodeURIComponent(id)}`, { method: 'DELETE' }),
}

// Labels follow the library shelves in assets/world/rooms.json (same keys as the backend enum).
export const shelves: { key: Shelf; label: string }[] = [
  { key: 'dongeng', label: 'Dongeng' },
  { key: 'novel', label: 'Novel' },
  { key: 'komik', label: 'Komik Edu' },
  { key: 'sains', label: 'Sains' },
  { key: 'matematika', label: 'Matematika' },
  { key: 'sejarah', label: 'Sejarah' },
  { key: 'inggris', label: 'Inggris' },
  { key: 'referensi', label: 'Referensi' },
]

export const formatLabel: Record<BookInput['format'], string> = { buku: 'Buku', ebook: 'E-book', audio: 'Audio' }
