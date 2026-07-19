export interface ApiEnvelope<T = unknown> {
  success: boolean
  message: string
  result?: T
}

export interface AuthResult {
  access_token: string
  token_type: string
  expires_in: number
  username: string
  role: string
}

export interface UserInfo {
  username: string
  role: string
  tenant?: string
  scopes?: string[]
}

export interface DeviceInfo {
  udid: string
  os: string
  name: string
  os_version: string
  provider: string
  usage: string
  screen_width: string
  screen_height: string
  device_type: string
  workspace_id: string
  stream_type: string
}

export interface HubDevice {
  info: DeviceInfo
  host: string
  connected: boolean
  provider_state: string
  available: boolean
  in_use: boolean
  in_use_by: string
  is_running_automation: boolean
}

export interface Workspace {
  id: string
  name: string
  description?: string
  tenant_id?: string
  is_default?: boolean
  device_count?: number
  user_count?: number
}

export interface Provider {
  os: string
  nickname: string
  host_address: string
  port: number
  provide_ios: boolean
  provide_android: boolean
  provide_tizen: boolean
  provide_webos: boolean
  setup_appium_servers: boolean
  wda_bundle_id?: string
  supervision_password?: string
  state?: string
  device_count?: number
}

export interface DBDevice {
  udid: string
  os: string
  name: string
  os_version: string
  provider: string
  usage: string
  screen_width: string
  screen_height: string
  device_type: string
  workspace_id: string
  stream_type: string
}

export interface AdminUser {
  username: string
  password?: string
  role: string
  workspace_ids?: string[]
}

export interface AdminFilesStatus {
  webdriveragent?: boolean
  supervision?: boolean
  broadcast?: boolean
  selenium?: boolean
  files?: { name: string; id: string }[]
}

export interface StreamSettings {
  target_fps: number
  jpeg_quality: number
  scaling_factor_android: number
  scaling_factor_ios: number
}

export interface TURNConfig {
  enabled: boolean
  server: string
  port: number
  shared_secret: string
  ttl: number
}

export interface SecretKey {
  id: string
  origin: string
  user_identifier_claim: string
  tenant_identifier_claim: string
  status: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface ClientCredential {
  id: string
  name: string
  description?: string
  client_id: string
  client_secret?: string
  workspace_id?: string
  created_at?: string
}

export interface CustomAction {
  id: string
  name: string
  description?: string
  action_type: string
  parameters?: Record<string, unknown>
  is_favorite?: boolean
}

export interface InstalledApp {
  bundleId?: string
  package?: string
  name?: string
}
