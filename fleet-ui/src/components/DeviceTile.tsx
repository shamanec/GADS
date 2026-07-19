import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  deviceHome,
  devicePressKey,
  deviceSwipe,
  deviceTap,
  deviceUploadApp,
  streamUrl,
} from '../api/devices'
import type { HubDevice } from '../api/types'
import {
  getDeviceStatus,
  KEYCODE_BACK,
  KEYCODE_VOLUME_DOWN,
  KEYCODE_VOLUME_UP,
  STATUS_COLORS,
} from '../utils/deviceHelpers'
import { useToast } from './ToastContext'

interface DeviceTileProps {
  device: HubDevice
  selected: boolean
  onToggleSelect: () => void
}

export function DeviceTile({ device, selected, onToggleSelect }: DeviceTileProps) {
  const navigate = useNavigate()
  const { toast } = useToast()
  const screenRef = useRef<HTMLDivElement>(null)
  const [visible, setVisible] = useState(false)
  const [dragOver, setDragOver] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)
  const dragStart = useRef<{ x: number; y: number } | null>(null)

  const info = device.info
  const status = getDeviceStatus(device)
  const offline = status === 'offline'
  const udid = info.udid

  useEffect(() => {
    const el = screenRef.current
    if (!el) return
    const obs = new IntersectionObserver(
      ([entry]) => setVisible(entry.isIntersecting),
      { rootMargin: '100px' },
    )
    obs.observe(el)
    return () => obs.disconnect()
  }, [])

  const mapCoords = (clientX: number, clientY: number) => {
    const rect = screenRef.current!.getBoundingClientRect()
    const sw = parseInt(info.screen_width, 10) || 1080
    const sh = parseInt(info.screen_height, 10) || 2400
    const x = Math.round(((clientX - rect.left) / rect.width) * sw)
    const y = Math.round(((clientY - rect.top) / rect.height) * sh)
    return { x, y }
  }

  const ripple = (e: React.MouseEvent) => {
    const el = screenRef.current
    if (!el) return
    const r = el.getBoundingClientRect()
    const s = document.createElement('span')
    s.className = 'ripple'
    s.style.left = `${e.clientX - r.left}px`
    s.style.top = `${e.clientY - r.top}px`
    el.appendChild(s)
    setTimeout(() => s.remove(), 480)
  }

  const handleScreenClick = async (e: React.MouseEvent) => {
    if (offline) {
      toast(`${info.name} is offline`)
      return
    }
    ripple(e)
    const { x, y } = mapCoords(e.clientX, e.clientY)
    try {
      await deviceTap(udid, x, y)
      toast(`Tap → ${info.name}`, 'ti-hand-finger')
    } catch (err) {
      toast(`Tap failed on ${info.name}: ${err instanceof Error ? err.message : 'error'}`)
    }
  }

  const handleMouseDown = (e: React.MouseEvent) => {
    if (offline) return
    dragStart.current = { x: e.clientX, y: e.clientY }
  }

  const handleMouseUp = async (e: React.MouseEvent) => {
    if (!dragStart.current || offline) return
    const dx = e.clientX - dragStart.current.x
    const dy = e.clientY - dragStart.current.y
    dragStart.current = null
    if (Math.abs(dx) < 8 && Math.abs(dy) < 8) return
    const start = mapCoords(e.clientX - dx, e.clientY - dy)
    const end = mapCoords(e.clientX, e.clientY)
    try {
      await deviceSwipe(udid, start.x, start.y, end.x, end.y)
    } catch {
      /* swipe may fail silently on some devices */
    }
  }

  const handleKey = async (action: string, keycode?: number) => {
    if (offline) {
      toast(`${info.name} is offline`)
      return
    }
    try {
      if (action === 'Home') {
        await deviceHome(udid)
        toast(`Home → ${info.name}`)
      } else if (keycode !== undefined) {
        await devicePressKey(udid, keycode)
        toast(`${action} → ${info.name}`)
      }
    } catch (err) {
      toast(
        `${action} failed on ${info.name}: ${err instanceof Error ? err.message : 'no endpoint'}`,
      )
    }
  }

  const installFile = async (file: File) => {
    try {
      await deviceUploadApp(udid, file)
      toast(`Installing ${file.name} → ${info.name}`, 'ti-upload')
    } catch (err) {
      toast(`Install failed on ${info.name}: ${err instanceof Error ? err.message : 'error'}`)
    }
  }

  return (
    <div className={`tile ${selected ? 'sel' : ''} ${offline ? 'offline' : ''}`}>
      <div className="th">
        <span className="dot" style={{ background: STATUS_COLORS[status] }} />
        <button
          type="button"
          className="nm"
          onClick={() => navigate(`/device/${udid}`)}
        >
          {info.name}
        </button>
        {!offline && (
          <span className="bat">
            <i className="ti ti-battery-4" />—
          </span>
        )}
        <button
          type="button"
          className="menu"
          title="Device settings"
          onClick={() => navigate(`/device/${udid}`)}
        >
          <i className="ti ti-dots-vertical" />
        </button>
      </div>
      <div
        ref={screenRef}
        className="screen"
        title="Click to tap · drag to swipe · double-click to open"
        onClick={handleScreenClick}
        onMouseDown={handleMouseDown}
        onMouseUp={handleMouseUp}
        onDoubleClick={() => navigate(`/device/${udid}`)}
        onDragOver={(e) => {
          e.preventDefault()
          if (!offline) setDragOver(true)
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={(e) => {
          e.preventDefault()
          setDragOver(false)
          const f = e.dataTransfer.files[0]
          if (f) void installFile(f)
        }}
      >
        <div className="scr">
          {offline ? (
            <div className="scr-off">
              <i className="ti ti-plug-connected-x" style={{ fontSize: 18 }} />
              offline
            </div>
          ) : visible ? (
            <img src={streamUrl(udid, info.os)} alt={info.name} />
          ) : (
            <div className="scr-off">…</div>
          )}
        </div>
        {dragOver && !offline && (
          <div className="dropov">
            <i className="ti ti-upload" style={{ fontSize: 20 }} />
            Drop to install on {info.name}
          </div>
        )}
      </div>
      <div className="ctrl">
        <button
          type="button"
          className="cb"
          title="Back"
          onClick={() => void handleKey('Back', KEYCODE_BACK)}
        >
          <i className="ti ti-arrow-left" />
        </button>
        <button type="button" className="cb" title="Home" onClick={() => void handleKey('Home')}>
          <i className="ti ti-home" />
        </button>
        <button
          type="button"
          className="cb"
          title="Volume down"
          onClick={() => void handleKey('Volume −', KEYCODE_VOLUME_DOWN)}
        >
          <i className="ti ti-volume-2" />
        </button>
        <button
          type="button"
          className="cb"
          title="Volume up"
          onClick={() => void handleKey('Volume +', KEYCODE_VOLUME_UP)}
        >
          <i className="ti ti-volume" />
        </button>
        <button
          type="button"
          className="cb"
          title="Install app"
          onClick={() => fileRef.current?.click()}
        >
          <i className="ti ti-upload" />
        </button>
        <input
          ref={fileRef}
          type="file"
          accept=".apk,.ipa"
          style={{ display: 'none' }}
          onChange={(e) => {
            const f = e.target.files?.[0]
            if (f) void installFile(f)
            e.target.value = ''
          }}
        />
      </div>
      <div className="tf">
        <i
          className={`ti ${info.os.toLowerCase() === 'ios' ? 'ti-brand-apple' : 'ti-brand-android'} osi`}
        />
        <span className="pv">{info.provider}</span>
        <button
          type="button"
          className={`chk ${selected ? 'on' : ''}`}
          title="Select for broadcast"
          onClick={onToggleSelect}
        >
          <i className="ti ti-check" />
        </button>
      </div>
    </div>
  )
}
