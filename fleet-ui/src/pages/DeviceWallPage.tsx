import { useEffect, useMemo, useState } from 'react'
import {
  connectDevicesSSE,
  deviceHome,
  deviceTap,
  deviceTypeText,
  deviceUploadApp,
  getWorkspaces,
} from '../api/devices'
import type { HubDevice, Workspace } from '../api/types'
import { DeviceTile } from '../components/DeviceTile'
import { Sidebar } from '../components/Sidebar'
import { useToast } from '../components/ToastContext'
import { getDeviceStatus } from '../utils/deviceHelpers'

type Filter = 'all' | 'ios' | 'android' | 'free' | 'inuse' | 'offline'

const VIRTUALIZE_THRESHOLD = 30

export function DeviceWallPage() {
  const { toast } = useToast()
  const [devices, setDevices] = useState<HubDevice[]>([])
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [workspaceId, setWorkspaceId] = useState('')
  const [filter, setFilter] = useState<Filter>('all')
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())

  useEffect(() => {
    getWorkspaces()
      .then((ws) => {
        setWorkspaces(ws)
        const def = ws.find((w) => w.is_default) ?? ws[0]
        if (def) setWorkspaceId(def.id)
      })
      .catch(() => toast('Failed to load workspaces'))
  }, [toast])

  useEffect(() => {
    if (!workspaceId) return
    const es = connectDevicesSSE(workspaceId, setDevices)
    return () => es.close()
  }, [workspaceId])

  const filtered = useMemo(() => {
    return devices.filter((d) => {
      const st = getDeviceStatus(d)
      if (filter === 'ios' && d.info.os.toLowerCase() !== 'ios') return false
      if (filter === 'android' && d.info.os.toLowerCase() !== 'android') return false
      if (filter === 'free' && st !== 'free') return false
      if (filter === 'inuse' && st !== 'inuse') return false
      if (filter === 'offline' && st !== 'offline') return false
      if (query && !d.info.name.toLowerCase().includes(query.toLowerCase())) return false
      return true
    })
  }, [devices, filter, query])

  const onlineCount = devices.filter((d) => getDeviceStatus(d) !== 'offline').length

  const toggleSelect = (udid: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(udid)) next.delete(udid)
      else next.add(udid)
      return next
    })
  }

  const broadcast = async (action: 'tap' | 'type' | 'home' | 'install') => {
    const list = devices.filter((d) => selected.has(d.info.udid))
    if (!list.length) return

    if (action === 'type') {
      const text = window.prompt('Text to type on all selected devices:')
      if (!text) return
      await Promise.allSettled(
        list.map(async (d) => {
          try {
            await deviceTypeText(d.info.udid, text)
            toast(`${d.info.name}: typed`, 'ti-check')
          } catch {
            toast(`${d.info.name}: type failed`, 'ti-x')
          }
        }),
      )
      return
    }

    if (action === 'install') {
      const input = document.createElement('input')
      input.type = 'file'
      input.accept = '.apk,.ipa'
      input.onchange = async () => {
        const file = input.files?.[0]
        if (!file) return
        await Promise.allSettled(
          list.map(async (d) => {
            try {
              await deviceUploadApp(d.info.udid, file)
              toast(`${d.info.name}: install ok`, 'ti-upload')
            } catch {
              toast(`${d.info.name}: install failed`, 'ti-x')
            }
          }),
        )
      }
      input.click()
      return
    }

    if (action === 'home') {
      await Promise.allSettled(
        list.map(async (d) => {
          try {
            await deviceHome(d.info.udid)
            toast(`${d.info.name}: home`, 'ti-check')
          } catch {
            toast(`${d.info.name}: home failed`, 'ti-x')
          }
        }),
      )
      return
    }

    await Promise.allSettled(
      list.map(async (d) => {
        const sw = parseInt(d.info.screen_width, 10) || 1080
        const sh = parseInt(d.info.screen_height, 10) || 2400
        try {
          await deviceTap(d.info.udid, Math.round(sw / 2), Math.round(sh / 2))
          toast(`${d.info.name}: tap`, 'ti-hand-finger')
        } catch {
          toast(`${d.info.name}: tap failed`, 'ti-x')
        }
      }),
    )
  }

  return (
    <>
      <Sidebar active="devices" />
      <div className="main">
        <header className="top">
          <h1>Devices</h1>
          <span className="count">
            {devices.length} devices · {onlineCount} online
          </span>
          <div className="search">
            <i className="ti ti-search" />
            <input
              placeholder="Search devices…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          <nav className="chips">
            {(
              [
                ['all', 'All'],
                ['ios', 'iOS'],
                ['android', 'Android'],
                ['free', 'Free'],
                ['inuse', 'In use'],
                ['offline', 'Offline'],
              ] as const
            ).map(([k, label]) => (
              <button
                key={k}
                type="button"
                className={`chip ${filter === k ? 'on' : ''}`}
                onClick={() => setFilter(k)}
              >
                {label}
              </button>
            ))}
          </nav>
          <select
            className="sfi"
            title="Workspace"
            value={workspaceId}
            onChange={(e) => setWorkspaceId(e.target.value)}
          >
            {workspaces.map((w) => (
              <option key={w.id} value={w.id}>
                {w.name}
              </option>
            ))}
          </select>
        </header>
        <section className="wall">
          {filtered.length === 0 ? (
            <div className="empty">No devices match.</div>
          ) : filtered.length > VIRTUALIZE_THRESHOLD ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              {Array.from({ length: Math.ceil(filtered.length / 6) }).map((_, rowIdx) => (
                <div
                  key={rowIdx}
                  className="grid"
                  style={{ gridTemplateColumns: 'repeat(6, 1fr)' }}
                >
                  {filtered.slice(rowIdx * 6, rowIdx * 6 + 6).map((d) => (
                    <DeviceTile
                      key={d.info.udid}
                      device={d}
                      selected={selected.has(d.info.udid)}
                      onToggleSelect={() => toggleSelect(d.info.udid)}
                    />
                  ))}
                </div>
              ))}
            </div>
          ) : (
            <div className="grid">
              {filtered.map((d) => (
                <DeviceTile
                  key={d.info.udid}
                  device={d}
                  selected={selected.has(d.info.udid)}
                  onToggleSelect={() => toggleSelect(d.info.udid)}
                />
              ))}
            </div>
          )}
        </section>
        <div className={`bcast ${selected.size > 0 ? 'show' : ''}`}>
          <i className="ti ti-broadcast" style={{ fontSize: 17 }} />
          <span className="n">
            Acting on {selected.size} device{selected.size === 1 ? '' : 's'}
          </span>
          <span style={{ flex: '0 0 10px' }} />
          <button type="button" className="bb" onClick={() => void broadcast('tap')}>
            <i className="ti ti-hand-finger" />
            Tap
          </button>
          <button type="button" className="bb" onClick={() => void broadcast('type')}>
            <i className="ti ti-keyboard" />
            Type
          </button>
          <button type="button" className="bb" onClick={() => void broadcast('home')}>
            <i className="ti ti-home" />
            Home
          </button>
          <button type="button" className="bb" onClick={() => void broadcast('install')}>
            <i className="ti ti-upload" />
            Install app
          </button>
          <button type="button" className="bclear" onClick={() => setSelected(new Set())}>
            Clear
          </button>
        </div>
      </div>
    </>
  )
}
