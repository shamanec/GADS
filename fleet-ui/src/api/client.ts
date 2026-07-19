import { clearToken, getToken } from './token'
import type { ApiEnvelope } from './types'

export class ApiError extends Error {
  status: number
  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

function redirectLogin(): void {
  clearToken()
  if (window.location.pathname !== '/login') {
    window.location.href = '/login'
  }
}

export async function authFetch<T = unknown>(
  path: string,
  opts: RequestInit = {},
): Promise<ApiEnvelope<T>> {
  const token = getToken()
  const headers = new Headers(opts.headers)
  if (token) headers.set('Authorization', `Bearer ${token}`)
  if (opts.body && !(opts.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const res = await fetch(path, { ...opts, headers })

  if (res.status === 401) {
    redirectLogin()
    throw new ApiError('Unauthorized', 401)
  }

  const contentType = res.headers.get('content-type') ?? ''
  if (!contentType.includes('application/json')) {
    if (!res.ok) throw new ApiError(res.statusText, res.status)
    return { success: true, message: 'ok', result: undefined as T }
  }

  const data = (await res.json()) as ApiEnvelope<T>
  if (!res.ok || data.success === false) {
    throw new ApiError(data.message || res.statusText, res.status)
  }
  return data
}

export async function authFetchRaw(path: string, opts: RequestInit = {}): Promise<Response> {
  const token = getToken()
  const headers = new Headers(opts.headers)
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const res = await fetch(path, { ...opts, headers })
  if (res.status === 401) {
    redirectLogin()
    throw new ApiError('Unauthorized', 401)
  }
  return res
}
