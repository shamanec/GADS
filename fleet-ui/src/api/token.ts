const TOKEN_KEY = 'gads_token'
const USERNAME_KEY = 'gads_username'

let memoryToken: string | null = null
let memoryUsername: string | null = null

export function getToken(): string | null {
  if (memoryToken) return memoryToken
  const stored = sessionStorage.getItem(TOKEN_KEY)
  if (stored) memoryToken = stored
  return memoryToken
}

export function setToken(token: string, username: string): void {
  memoryToken = token
  memoryUsername = username
  sessionStorage.setItem(TOKEN_KEY, token)
  sessionStorage.setItem(USERNAME_KEY, username)
}

export function getUsername(): string | null {
  if (memoryUsername) return memoryUsername
  const stored = sessionStorage.getItem(USERNAME_KEY)
  if (stored) memoryUsername = stored
  return memoryUsername
}

export function clearToken(): void {
  memoryToken = null
  memoryUsername = null
  sessionStorage.removeItem(TOKEN_KEY)
  sessionStorage.removeItem(USERNAME_KEY)
}
