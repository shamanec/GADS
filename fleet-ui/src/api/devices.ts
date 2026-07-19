import { authFetch, authFetchRaw } from './client'
import { getToken } from './token'
import type { HubDevice, InstalledApp, Workspace } from './types'

export async function getWorkspaces() {
  const data = await authFetch<Workspace[]>('/workspaces')
  return data.result ?? []
}

export function streamUrl(udid: string, os: string): string {
  const token = getToken()
  const path =
    os.toLowerCase() === 'android'
      ? `/device/${udid}/android-stream-mjpeg`
      : `/device/${udid}/ios-stream-mjpeg`
  return token ? `${path}?token=${encodeURIComponent(token)}` : path
}

export async function deviceTap(udid: string, x: number, y: number) {
  return authFetch(`/device/${udid}/tap`, {
    method: 'POST',
    body: JSON.stringify({ x, y }),
  })
}

export async function deviceSwipe(
  udid: string,
  x: number,
  y: number,
  endX: number,
  endY: number,
) {
  return authFetch(`/device/${udid}/swipe`, {
    method: 'POST',
    body: JSON.stringify({ x, y, endX, endY }),
  })
}

export async function deviceHome(udid: string) {
  return authFetch(`/device/${udid}/home`, { method: 'POST' })
}

export async function deviceRecents(udid: string) {
  return authFetch(`/device/${udid}/recents`, { method: 'POST' })
}

export async function deviceTypeText(udid: string, text: string) {
  return authFetch(`/device/${udid}/typeText`, {
    method: 'POST',
    body: JSON.stringify({ text }),
  })
}

export async function deviceScreenshot(udid: string) {
  const res = await authFetchRaw(`/device/${udid}/screenshot`, { method: 'POST' })
  return res.blob()
}

export async function deviceReset(udid: string) {
  return authFetch(`/device/${udid}/reset`, { method: 'POST' })
}

export async function deviceGetClipboard(udid: string) {
  const data = await authFetch<string>(`/device/${udid}/getClipboard`)
  return data.result ?? ''
}

export async function deviceRotation(udid: string, rotation: string) {
  return authFetch(`/device/${udid}/rotation`, {
    method: 'POST',
    body: JSON.stringify({ rotation }),
  })
}

export async function deviceUploadApp(udid: string, file: File) {
  const form = new FormData()
  form.append('file', file)
  return authFetch(`/device/${udid}/uploadAndInstallApp`, {
    method: 'POST',
    body: form,
  })
}

export async function deviceUninstallApp(udid: string, appId: string) {
  return authFetch(`/device/${udid}/uninstallApp`, {
    method: 'POST',
    body: JSON.stringify({ appId }),
  })
}

export async function deviceApps(udid: string) {
  const data = await authFetch<InstalledApp[]>(`/device/${udid}/apps`)
  return data.result ?? []
}

export async function lockDevice(udid: string, ttlMinutes = 10) {
  return authFetch(`/devices/control/${udid}/lock?ttl_minutes=${ttlMinutes}`, {
    method: 'POST',
  })
}

export async function unlockDevice(udid: string) {
  return authFetch(`/devices/control/${udid}/unlock`, { method: 'POST' })
}

/** Attempt back/volume via pressKey if provider supports it; may fail — see QUESTIONS.md */
export async function devicePressKey(udid: string, keycode: number) {
  return authFetch(`/device/${udid}/pressKey`, {
    method: 'POST',
    body: JSON.stringify({ keycode }),
  })
}

export function connectDevicesSSE(
  workspaceId: string,
  onDevices: (devices: HubDevice[]) => void,
  onError?: (err: Event) => void,
): EventSource {
  const token = getToken()
  const url = `/available-devices?workspaceId=${encodeURIComponent(workspaceId)}${token ? `&token=${encodeURIComponent(token)}` : ''}`
  const es = new EventSource(url)
  es.onmessage = (ev) => {
    try {
      const devices = JSON.parse(ev.data) as HubDevice[]
      onDevices(devices)
    } catch {
      /* ignore parse errors */
    }
  }
  if (onError) es.onerror = onError
  return es
}
