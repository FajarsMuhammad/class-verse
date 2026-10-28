export type BackendStatus = 'ok' | 'down'

// checkHealth reports whether the Go server answers GET /healthz with {"status":"ok"}.
export async function checkHealth(signal?: AbortSignal): Promise<BackendStatus> {
  try {
    const res = await fetch('/healthz', { signal })
    if (!res.ok) return 'down'
    const body: unknown = await res.json()
    return isOk(body) ? 'ok' : 'down'
  } catch {
    return 'down'
  }
}

function isOk(body: unknown): boolean {
  return typeof body === 'object' && body !== null && (body as { status?: unknown }).status === 'ok'
}
