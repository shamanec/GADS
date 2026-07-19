import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  connectDevicesSSE,
  deviceApps,
  deviceGetClipboard,
  deviceHome,
  devicePressKey,
  deviceReset,
  deviceRotation,
  deviceScreenshot,
  deviceTap,
  deviceUninstallApp,
  deviceUploadApp,
  getWorkspaces,
  lockDevice,
  streamUrl,
  unlockDevice,
} from '../api/devices'
import type { HubDevice, InstalledApp, Workspace } from '../api/types'
import { Sidebar } from '../components/Sidebar'
import { useToast } from '../components/ToastContext'
import {
  getDeviceStatus,
  KEYCODE_BACK,
  KEYCODE_VOLUME_DOWN,
  KEYCODE_VOLUME_UP,
  STATUS_COLORS,
  STATUS_LABELS,
} from '../utils/deviceHelpers'

export function DeviceDetailPage() {
  const { udid = '' } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const [device, setDevice] = useState<HubDevice | null>(null)
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [apps, setApps] = useState<InstalledApp[]>([])
  const [reserved, setReserved] = useState(false)
  const [protocol, setProtocol] = useState('WebRTC')
  const [quality, setQuality] = useState('High')

  const loadApps = useCallback(async () => {
    if (!udid) return
    try {
      const list = await deviceApps(udid)
      setApps(list)
    } catch {
      setApps([])
    }
  }, [udid])

  useEffect(() => {
    getWorkspaces().then(setWorkspaces).catch(() => {})
  }, [])

  useEffect(() => {
    if (!udid) return
    const ws = workspaces[0]
    if (!ws) return
    const es = connectDevicesSSE(ws.id, (list) => {
      const found = list.find((d) => d.info.udid === udid)
      if (found) setDevice(found)
    })
    return () => es.close()
  }, [udid, workspaces])

  useEffect(() => {
    void loadApps()
  }, [loadApps])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') navigate('/')
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [navigate])

  if (!device) {
    return (
      <>
        <Sidebar active="devices" />
        <div className="main detail-page">
          <div className="empty">Loading device…</div>
        </div>
      </>
    )
  }

  const info = device.info
  const status = getDeviceStatus(device)
  const offline = status === 'offline'
  const wsName =
    workspaces.find((w) => w.id === info.workspace_id)?.name ?? info.workspace_id

  const mapCoords = (clientX: number, clientY: number, rect: DOMRect) => {
    const sw = parseInt(info.screen_width, 10) || 1080
    const sh = parseInt(info.screen_height, 10) || 2400
    return {
      x: Math.round(((clientX - rect.left) / rect.width) * sw),
      y: Math.round(((clientY - rect.top) / rect.height) * sh),
    }
  }

  const handleTap = async (e: React.MouseEvent<HTMLDivElement>) => {
    if (offline) {
      toast(`${info.name} is offline`)
      return
    }
    const rect = e.currentTarget.getBoundingClientRect()
    const { x, y } = mapCoords(e.clientX, e.clientY, rect)
    try {
      await deviceTap(udid, x, y)
      toast(`Tap → ${info.name}`, 'ti-hand-finger')
    } catch (err) {
      toast(`Tap failed: ${err instanceof Error ? err.message : 'error'}`)
    }
  }

  const hwAction = async (action: string, keycode?: number) => {
    if (offline) {
      toast(`${info.name} is offline`)
      return
    }
    try {
      if (action === 'Home') await deviceHome(udid)
      else if (action === 'Recents') await deviceHome(udid)
      else if (keycode !== undefined) await devicePressKey(udid, keycode)
      toast(`${action} → ${info.name}`)
    } catch (err) {
      toast(`${action} failed: ${err instanceof Error ? err.message : 'no endpoint'}`)
    }
  }

  const pillStyle = {
    background:
      status === 'free'
        ? '#e7f6ec'
        : status === 'inuse'
          ? 'var(--acc-soft)'
          : status === 'reserved'
            ? '#fdf3e4'
            : '#eef0f2',
    color: STATUS_COLORS[status],
  }

  return (
    <>
      <Sidebar
        active="devices"
      />
      <div className="main detail-page">
        <div className="dtop">
          <button type="button" className="backb" onClick={() => navigate('/')}>
            <i className="ti ti-arrow-left" />
            Fleet
          </button>
          <h2>{info.name}</h2>
          <span className="pill" style={pillStyle}>
            {STATUS_LABELS[status]}
          </span>
          <span style={{ flex: 1 }} />
          <span style={{ fontSize: '12.5px', color: 'var(--tx3)' }}>
            provider {info.provider} · {info.os} {info.os_version}
          </span>
        </div>
        <div className="dbody">
          <div className="dleft">
            <div className="bigscreen" onClick={handleTap}>
              <div className="bigscr">
                {offline ? (
                  <div className="scr-off">
                    <i className="ti ti-plug-connected-x" style={{ fontSize: 26 }} />
                    offline
                  </div>
                ) : (
                  <img src={streamUrl(udid, info.os)} alt={info.name} />
                )}
              </div>
            </div>
            <div className="hw">
              <button
                type="button"
                className="hb"
                title="Back"
                onClick={() => void hwAction('Back', KEYCODE_BACK)}
              >
                <i className="ti ti-arrow-left" />
              </button>
              <button type="button" className="hb" title="Home" onClick={() => void hwAction('Home')}>
                <i className="ti ti-home" />
              </button>
              <button
                type="button"
                className="hb"
                title="Recents"
                onClick={() => void hwAction('Recents')}
              >
                <i className="ti ti-square" />
              </button>
              <button
                type="button"
                className="hb"
                title="Volume down"
                onClick={() => void hwAction('Volume −', KEYCODE_VOLUME_DOWN)}
              >
                <i className="ti ti-volume-2" />
              </button>
              <button
                type="button"
                className="hb"
                title="Volume up"
                onClick={() => void hwAction('Volume +', KEYCODE_VOLUME_UP)}
              >
                <i className="ti ti-volume" />
              </button>
              <button
                type="button"
                className="hb"
                title="Power"
                onClick={() => toast('Power → not supported via API')}
              >
                <i className="ti ti-power" />
              </button>
            </div>
            <div className="dact">
              <button
                type="button"
                className="ab"
                onClick={async () => {
                  try {
                    const blob = await deviceScreenshot(udid)
                    const url = URL.createObjectURL(blob)
                    const a = document.createElement('a')
                    a.href = url
                    a.download = `${info.name}-screenshot.png`
                    a.click()
                    URL.revokeObjectURL(url)
                    toast('Screenshot saved')
                  } catch {
                    toast('Screenshot failed')
                  }
                }}
              >
                <i className="ti ti-camera" />
                Screenshot
              </button>
              <button
                type="button"
                className="ab"
                onClick={async () => {
                  try {
                    await deviceRotation(udid, '90')
                    toast('Rotated')
                  } catch {
                    toast('Rotate failed')
                  }
                }}
              >
                <i className="ti ti-rotate" />
                Rotate
              </button>
            </div>
          </div>
          <div className="dright">
            <div className="card">
              <h3>
                <i className="ti ti-info-circle" />
                Device
              </h3>
              <div className="krow">
                <span className="k">Model</span>
                <span>
                  {info.name} · {info.os} {info.os_version}
                </span>
              </div>
              <div className="krow">
                <span className="k">UDID</span>
                <span className="mono">{info.udid}</span>
              </div>
              <div className="krow">
                <span className="k">Screen</span>
                <span>
                  {info.screen_width} × {info.screen_height}
                </span>
              </div>
              <div className="krow">
                <span className="k">Appium port</span>
                <span className="mono">—</span>
              </div>
              <div className="krow">
                <span className="k">Workspace</span>
                <span>{wsName}</span>
              </div>
              <div className="krow">
                <span className="k">Appium configuration</span>
                <button
                  type="button"
                  style={{ color: 'var(--acc)', fontSize: 13 }}
                  onClick={() => {
                    const caps = JSON.stringify(
                      {
                        platformName: info.os,
                        'appium:udid': info.udid,
                        'appium:deviceName': info.name,
                      },
                      null,
                      2,
                    )
                    void navigator.clipboard.writeText(caps)
                    toast('Appium capabilities copied')
                  }}
                >
                  Copy <i className="ti ti-copy" style={{ fontSize: 12 }} />
                </button>
              </div>
              <div className="krow">
                <span className="k">Reserved by you</span>
                <button
                  type="button"
                  className={`tog ${reserved ? 'on' : ''}`}
                  onClick={async () => {
                    try {
                      if (reserved) {
                        await unlockDevice(udid)
                        setReserved(false)
                        toast(`${info.name} released`)
                      } else {
                        await lockDevice(udid)
                        setReserved(true)
                        toast(`${info.name} reserved`)
                      }
                    } catch (err) {
                      toast(err instanceof Error ? err.message : 'Lock failed')
                    }
                  }}
                />
              </div>
            </div>

            <div className="card">
              <h3>
                <i className="ti ti-apps" />
                Apps
              </h3>
              <div
                className="dz"
                onClick={() => {
                  const input = document.createElement('input')
                  input.type = 'file'
                  input.accept = info.os.toLowerCase() === 'ios' ? '.ipa' : '.apk'
                  input.onchange = async () => {
                    const f = input.files?.[0]
                    if (f) {
                      try {
                        await deviceUploadApp(udid, f)
                        toast(`Installing ${f.name}`, 'ti-upload')
                        void loadApps()
                      } catch {
                        toast('Install failed')
                      }
                    }
                  }
                  input.click()
                }}
                onDragOver={(e) => {
                  e.preventDefault()
                  e.currentTarget.classList.add('over')
                }}
                onDragLeave={(e) => e.currentTarget.classList.remove('over')}
                onDrop={async (e) => {
                  e.preventDefault()
                  e.currentTarget.classList.remove('over')
                  const f = e.dataTransfer.files[0]
                  if (f) {
                    try {
                      await deviceUploadApp(udid, f)
                      toast(`Installing ${f.name}`, 'ti-upload')
                      void loadApps()
                    } catch {
                      toast('Install failed')
                    }
                  }
                }}
              >
                <i className="ti ti-upload" style={{ fontSize: 20 }} />
                Drag {info.os.toLowerCase() === 'ios' ? '.ipa' : '.apk'} here, or click to browse
              </div>
              {apps.length === 0 ? (
                <div style={{ fontSize: '12.5px', color: 'var(--tx3)', padding: '8px 0' }}>
                  No user apps installed.
                </div>
              ) : (
                apps.map((a, i) => {
                  const id = a.bundleId || a.package || a.name || `app-${i}`
                  return (
                    <div key={id} className="aprow">
                      <span className="api">
                        <i className="ti ti-package" />
                      </span>
                      <span className="mono" style={{ flex: 1 }}>
                        {id}
                      </span>
                      <button
                        type="button"
                        className="tb"
                        title="Uninstall"
                        onClick={async () => {
                          try {
                            await deviceUninstallApp(udid, id)
                            toast(`Uninstalled ${id}`)
                            void loadApps()
                          } catch {
                            toast('Uninstall failed')
                          }
                        }}
                      >
                        <i className="ti ti-trash" />
                      </button>
                    </div>
                  )
                })
              )}
            </div>

            <div className="card">
              <h3>
                <i className="ti ti-video" />
                Streaming
              </h3>
              <div className="krow">
                <span className="k">Protocol</span>
                <span className="seg">
                  <button
                    type="button"
                    className={protocol === 'WebRTC' ? 'on' : ''}
                    onClick={() => setProtocol('WebRTC')}
                  >
                    WebRTC
                  </button>
                  <button
                    type="button"
                    className={protocol === 'MJPEG' ? 'on' : ''}
                    onClick={() => setProtocol('MJPEG')}
                  >
                    MJPEG
                  </button>
                </span>
              </div>
              <div className="krow">
                <span className="k">Quality</span>
                <span className="seg">
                  <button
                    type="button"
                    className={quality === 'Low' ? 'on' : ''}
                    onClick={() => setQuality('Low')}
                  >
                    Low
                  </button>
                  <button
                    type="button"
                    className={quality === 'High' ? 'on' : ''}
                    onClick={() => setQuality('High')}
                  >
                    High
                  </button>
                </span>
              </div>
            </div>

            <div className="card">
              <h3>
                <i className="ti ti-settings" />
                Session &amp; actions
              </h3>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                <button
                  type="button"
                  className="sab"
                  onClick={async () => {
                    try {
                      const text = await deviceGetClipboard(udid)
                      await navigator.clipboard.writeText(text)
                      toast('Clipboard synced')
                    } catch {
                      toast('Clipboard sync failed')
                    }
                  }}
                >
                  <i className="ti ti-clipboard" />
                  Clipboard
                </button>
                <button
                  type="button"
                  className="sab"
                  onClick={() => toast('Appium session — use /grid endpoint')}
                >
                  <i className="ti ti-player-play" />
                  Appium session
                </button>
                <button type="button" className="sab" onClick={() => toast('Restarting…')}>
                  <i className="ti ti-refresh" />
                  Restart
                </button>
                <button
                  type="button"
                  className="sab warn"
                  onClick={async () => {
                    try {
                      await deviceReset(udid)
                      toast('Reset requested')
                    } catch {
                      toast('Reset failed')
                    }
                  }}
                >
                  <i className="ti ti-eraser" />
                  Reset
                </button>
              </div>
            </div>

            <div className="card">
              <h3>
                <i className="ti ti-grid-dots" />
                Selenium grid &amp; logs
              </h3>
              <div className="krow">
                <span className="k">Register as grid node</span>
                <button
                  type="button"
                  className="tog"
                  onClick={() => toast('No per-device register API — use /grid')}
                />
              </div>
              <div className="krow">
                <span className="k">Device &amp; Appium logs</span>
                <button
                  type="button"
                  style={{ color: 'var(--acc)', fontSize: 13 }}
                  onClick={() => window.open('/appium-logs', '_blank')}
                >
                  Open <i className="ti ti-external-link" style={{ fontSize: 12 }} />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </>
  )
}
