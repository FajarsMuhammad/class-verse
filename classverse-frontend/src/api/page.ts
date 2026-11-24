/** Page size of every Kelola list and picker. */
export const PAGE = 20

/** Whether a page has a successor: exact with a server total, else a full page may have one. */
export const hasMore = (offset: number, page: { items: unknown[]; total?: number }) =>
  page.total === undefined ? page.items.length === PAGE : offset + page.items.length < page.total

/**
 * The page to show after a load. An empty page past the end (the last item of the last page was removed)
 * steps back one page; the server cannot report a total for an offset past the end.
 */
export const lastPage = (page: number, list: { items: unknown[] }) => (!list.items.length && page > 1 ? page - 1 : page)
