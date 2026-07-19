import { authFetch } from './client'
import type {
  AdminFilesStatus,
  AdminUser,
  ClientCredential,
  CustomAction,
  DBDevice,
  Provider,
  SecretKey,
  StreamSettings,
  TURNConfig,
  Workspace,
} from './types'

// Providers
export async function getProviders() {
  const data = await authFetch<Provider[]>('/admin/providers')
  return data.result ?? []
}

export async function addProvider(provider: Provider) {
  return authFetch('/admin/providers/add', {
    method: 'POST',
    body: JSON.stringify(provider),
  })
}

export async function updateProvider(provider: Provider) {
  return authFetch('/admin/providers/update', {
    method: 'POST',
    body: JSON.stringify(provider),
  })
}

export async function deleteProvider(nickname: string) {
  return authFetch(`/admin/providers/${encodeURIComponent(nickname)}`, {
    method: 'DELETE',
  })
}

export async function getProviderLogs(collection = '', logLimit = 100) {
  const q = new URLSearchParams({ collection, logLimit: String(logLimit) })
  return authFetch(`/admin/providers/logs?${q}`)
}

// Devices
export async function getAdminDevices() {
  const data = await authFetch<{
    devices: DBDevice[]
    providers: Provider[]
    device_stream_types: string[]
  }>('/admin/devices')
  return data.result ?? { devices: [], providers: [], device_stream_types: [] }
}

export async function addDevice(device: DBDevice) {
  return authFetch('/admin/device', { method: 'POST', body: JSON.stringify(device) })
}

export async function updateDevice(device: DBDevice) {
  return authFetch('/admin/device', { method: 'PUT', body: JSON.stringify(device) })
}

export async function deleteDevice(udid: string) {
  return authFetch(`/admin/device/${encodeURIComponent(udid)}`, { method: 'DELETE' })
}

// Users
export async function getUsers() {
  const data = await authFetch<AdminUser[]>('/admin/users')
  return data.result ?? []
}

export async function addUser(user: AdminUser) {
  return authFetch('/admin/user', { method: 'POST', body: JSON.stringify(user) })
}

export async function updateUser(user: AdminUser) {
  return authFetch('/admin/user', { method: 'PUT', body: JSON.stringify(user) })
}

export async function deleteUser(username: string) {
  return authFetch(`/admin/user/${encodeURIComponent(username)}`, { method: 'DELETE' })
}

// Files
export async function getAdminFiles() {
  const data = await authFetch<AdminFilesStatus>('/admin/files')
  return data.result ?? {}
}

export async function uploadSupervision(file: File) {
  const form = new FormData()
  form.append('file', file)
  return authFetch('/admin/files/supervision', { method: 'POST', body: form })
}

export async function uploadWDA(file: File) {
  const form = new FormData()
  form.append('file', file)
  return authFetch('/admin/files/webdriveragent', { method: 'POST', body: form })
}

export async function uploadSeleniumJar(file: File) {
  const form = new FormData()
  form.append('file', file)
  return authFetch('/admin/files/selenium', { method: 'POST', body: form })
}

// Global settings
export async function getGlobalSettings() {
  const data = await authFetch<StreamSettings>('/admin/global-settings')
  return data.result
}

export async function saveGlobalSettings(settings: StreamSettings) {
  return authFetch('/admin/global-settings', {
    method: 'POST',
    body: JSON.stringify(settings),
  })
}

export async function getTurnConfig() {
  const data = await authFetch<TURNConfig>('/admin/turn-config')
  return data.result
}

export async function saveTurnConfig(config: TURNConfig) {
  return authFetch('/admin/turn-config', {
    method: 'POST',
    body: JSON.stringify(config),
  })
}

export async function getSystemStatus() {
  return authFetch('/admin/system-status')
}

// Workspaces
export async function getAdminWorkspaces() {
  const data = await authFetch<Workspace[]>('/admin/workspaces')
  return data.result ?? []
}

export async function addWorkspace(ws: { name: string; description?: string }) {
  return authFetch('/admin/workspaces', { method: 'POST', body: JSON.stringify(ws) })
}

export async function updateWorkspace(ws: Workspace) {
  return authFetch('/admin/workspaces', { method: 'PUT', body: JSON.stringify(ws) })
}

export async function deleteWorkspace(id: string) {
  return authFetch(`/admin/workspaces/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// Secret keys
export async function getSecretKeys() {
  const data = await authFetch<SecretKey[]>('/admin/secret-keys')
  return data.result ?? []
}

export async function addSecretKey(body: Record<string, unknown>) {
  return authFetch('/admin/secret-keys', { method: 'POST', body: JSON.stringify(body) })
}

export async function updateSecretKey(id: string, body: Record<string, unknown>) {
  return authFetch(`/admin/secret-keys/${id}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export async function disableSecretKey(id: string) {
  return authFetch(`/admin/secret-keys/${id}`, { method: 'DELETE' })
}

export async function getSecretKeyHistory() {
  return authFetch('/admin/secret-keys/history')
}

// Client credentials
export async function getClientCredentials() {
  const data = await authFetch<ClientCredential[]>('/client-credentials')
  return data.result ?? []
}

export async function createClientCredential(body: {
  name: string
  description?: string
}) {
  const data = await authFetch<ClientCredential>('/client-credentials', {
    method: 'POST',
    body: JSON.stringify(body),
  })
  return data.result!
}

export async function deleteClientCredential(id: string) {
  return authFetch(`/client-credentials/${id}`, { method: 'DELETE' })
}

// Custom actions
export async function getCustomActions() {
  const data = await authFetch<CustomAction[]>('/custom-actions')
  return data.result ?? []
}

export async function addCustomAction(body: Record<string, unknown>) {
  return authFetch('/custom-actions', { method: 'POST', body: JSON.stringify(body) })
}

export async function updateCustomAction(id: string, body: Record<string, unknown>) {
  return authFetch(`/custom-actions/${id}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export async function deleteCustomAction(id: string) {
  return authFetch(`/custom-actions/${id}`, { method: 'DELETE' })
}

export async function getFavoriteActions() {
  const data = await authFetch<CustomAction[]>('/custom-actions/favorites')
  return data.result ?? []
}

export async function addFavoriteAction(id: string) {
  return authFetch(`/custom-actions/favorites/${id}`, { method: 'POST' })
}

export async function removeFavoriteAction(id: string) {
  return authFetch(`/custom-actions/favorites/${id}`, { method: 'DELETE' })
}
