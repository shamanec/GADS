import { authFetch } from './client'
import type { AuthResult, UserInfo } from './types'

export async function authenticate(username: string, password: string) {
  const res = await fetch('/authenticate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  const data = await res.json()
  if (!res.ok || !data.success) {
    throw new Error(data.message || 'Authentication failed')
  }
  return data.result as AuthResult
}

export async function logout() {
  return authFetch('/logout', { method: 'POST' })
}

export async function userInfo() {
  const data = await authFetch<UserInfo>('/user-info')
  return data.result!
}
