export type DeviceStatus = 'free' | 'inuse' | 'reserved' | 'offline'

export function getDeviceStatus(d: {
  connected: boolean
  available: boolean
  in_use: boolean
  in_use_by?: string
}): DeviceStatus {
  if (!d.connected || !d.available) return 'offline'
  if (d.in_use) return 'inuse'
  return 'free'
}

export const STATUS_COLORS: Record<DeviceStatus, string> = {
  free: 'var(--ok)',
  inuse: 'var(--acc)',
  reserved: 'var(--warn)',
  offline: 'var(--tx3)',
}

export const STATUS_LABELS: Record<DeviceStatus, string> = {
  free: 'free',
  inuse: 'in use',
  reserved: 'reserved',
  offline: 'offline',
}

export function osLabel(os: string): string {
  const o = os.toLowerCase()
  if (o === 'ios') return 'iOS'
  if (o === 'android') return 'Android'
  return os
}

export function mapOsToBackend(label: string): string {
  const m: Record<string, string> = {
    iOS: 'ios',
    Android: 'android',
    Tizen: 'tizen',
    WebOS: 'webos',
    macOS: 'darwin',
    Linux: 'linux',
    Windows: 'windows',
  }
  return m[label] ?? label.toLowerCase()
}

export function mapOsFromBackend(os: string): string {
  const m: Record<string, string> = {
    ios: 'iOS',
    android: 'Android',
    tizen: 'Tizen',
    webos: 'WebOS',
    darwin: 'macOS',
    linux: 'Linux',
    windows: 'Windows',
  }
  return m[os.toLowerCase()] ?? os
}

export function mapBoolYesNo(v: boolean): string {
  return v ? 'Yes' : 'No'
}

export function mapYesNoBool(v: string): boolean {
  return v === 'Yes'
}

export const KEYCODE_BACK = 4
export const KEYCODE_VOLUME_DOWN = 25
export const KEYCODE_VOLUME_UP = 24
