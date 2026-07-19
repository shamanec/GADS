import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  addCustomAction,
  addDevice,
  addFavoriteAction,
  addProvider,
  addSecretKey,
  addUser,
  addWorkspace,
  createClientCredential,
  deleteClientCredential,
  deleteCustomAction,
  deleteDevice,
  deleteProvider,
  deleteUser,
  deleteWorkspace,
  disableSecretKey,
  getAdminDevices,
  getAdminFiles,
  getAdminWorkspaces,
  getClientCredentials,
  getCustomActions,
  getGlobalSettings,
  getProviders,
  getSecretKeyHistory,
  getSecretKeys,
  getSystemStatus,
  getTurnConfig,
  getUsers,
  removeFavoriteAction,
  saveGlobalSettings,
  saveTurnConfig,
  updateCustomAction,
  updateDevice,
  updateProvider,
  updateSecretKey,
  updateUser,
  updateWorkspace,
  uploadSeleniumJar,
  uploadSupervision,
  uploadWDA,
} from '../api/admin'
import type {
  ClientCredential,
  CustomAction,
  DBDevice,
  Provider,
  SecretKey,
  StreamSettings,
  TURNConfig,
  Workspace,
} from '../api/types'
import {
  FormModal,
  FormRow,
} from '../components/FormModal'
import { Sidebar } from '../components/Sidebar'
import { useToast } from '../components/ToastContext'
import { useAuth } from '../auth/AuthContext'
import {
  mapBoolYesNo,
  mapOsFromBackend,
  mapOsToBackend,
  mapYesNoBool,
} from '../utils/deviceHelpers'

const TAB_LABELS: Record<string, string> = {
  providers: 'Providers',
  devices: 'Devices',
  users: 'Users',
  files: 'Files',
  settings: 'Global settings',
  workspaces: 'Workspaces',
  keys: 'Secret keys',
  creds: 'Client credentials',
  actions: 'Custom actions',
}

export function AdminPage({ tab }: { tab: string }) {
  const navigate = useNavigate()
  const { toast } = useToast()
  const { username } = useAuth()
  const [modal, setModal] = useState<{
    title: string
    note?: string
    ok: string
    body: React.ReactNode
    onSubmit: () => void
  } | null>(null)

  const sidebarActive =
    tab === 'providers' ? 'providers' : tab === 'settings' ? 'settings' : 'admin'

  return (
    <>
      <Sidebar active={sidebarActive} />
      <div className="main admin-page">
        <div className="dtop">
          <button type="button" className="backb" onClick={() => navigate('/')}>
            <i className="ti ti-arrow-left" />
            Fleet
          </button>
          <h2>Administration</h2>
          <span style={{ flex: 1 }} />
          <span style={{ fontSize: '12.5px', color: 'var(--tx3)' }}>
            signed in as <b>{username}</b>
          </span>
        </div>
        <nav className="atabs">
          {Object.entries(TAB_LABELS).map(([k, label]) => (
            <button
              key={k}
              type="button"
              className={`atab ${tab === k ? 'on' : ''}`}
              onClick={() => navigate(`/admin/${k}`)}
            >
              {label}
            </button>
          ))}
        </nav>
        <div className="abody">
          {tab === 'providers' && (
            <ProvidersTab toast={toast} setModal={setModal} />
          )}
          {tab === 'devices' && <DevicesTab toast={toast} setModal={setModal} />}
          {tab === 'users' && <UsersTab toast={toast} setModal={setModal} />}
          {tab === 'files' && <FilesTab toast={toast} />}
          {tab === 'settings' && <SettingsTab toast={toast} />}
          {tab === 'workspaces' && (
            <WorkspacesTab toast={toast} setModal={setModal} />
          )}
          {tab === 'keys' && <KeysTab toast={toast} setModal={setModal} />}
          {tab === 'creds' && <CredsTab toast={toast} setModal={setModal} />}
          {tab === 'actions' && <ActionsTab toast={toast} setModal={setModal} />}
        </div>
      </div>
      {modal && (
        <FormModal
          open
          title={modal.title}
          note={modal.note}
          okLabel={modal.ok}
          onClose={() => setModal(null)}
          onSubmit={modal.onSubmit}
        >
          {modal.body}
        </FormModal>
      )}
    </>
  )
}

type ModalSetter = React.Dispatch<
  React.SetStateAction<{
    title: string
    note?: string
    ok: string
    body: React.ReactNode
    onSubmit: () => void
  } | null>
>

function ProvidersTab({
  toast,
  setModal,
}: {
  toast: (m: string, i?: string) => void
  setModal: ModalSetter
}) {
  const [providers, setProviders] = useState<Provider[]>([])
  const [deviceCounts, setDeviceCounts] = useState<Record<string, number>>({})

  const load = useCallback(async () => {
    const [p, d] = await Promise.all([getProviders(), getAdminDevices()])
    setProviders(p)
    const counts: Record<string, number> = {}
    for (const dev of d.devices) counts[dev.provider] = (counts[dev.provider] || 0) + 1
    setDeviceCounts(counts)
  }, [])

  useEffect(() => {
    load().catch(() => toast('Failed to load providers'))
  }, [load, toast])

  const openForm = (idx: number | null) => {
    const p = idx == null ? ({} as Provider) : providers[idx]
    const st = {
      os: mapOsFromBackend(p.os || 'darwin'),
      nickname: p.nickname || '',
      host: p.host_address || '',
      port: String(p.port || 10001),
      ios: mapBoolYesNo(p.provide_ios ?? false),
      android: mapBoolYesNo(p.provide_android ?? false),
      tizen: mapBoolYesNo(p.provide_tizen ?? false),
      webos: mapBoolYesNo(p.provide_webos ?? false),
      appium: mapBoolYesNo(p.setup_appium_servers ?? false),
      wda: p.wda_bundle_id || '',
      sup: p.supervision_password || '',
    }
    setModal({
      title: idx == null ? 'Add provider' : 'Update provider',
      note: 'All updates to existing provider config require provider instance restart.',
      ok: idx == null ? 'Add provider' : 'Update provider',
      body: (
        <ProviderFormFields st={st} />
      ),
      onSubmit: async () => {
        const el = (id: string) => (document.getElementById(id) as HTMLInputElement)?.value.trim()
        const body: Provider = {
          os: mapOsToBackend(el('f_pos') || 'macOS'),
          nickname: el('f_nick') || '',
          host_address: el('f_host') || '',
          port: parseInt(el('f_port') || '10001', 10),
          provide_ios: mapYesNoBool(el('f_ios') || 'No'),
          provide_android: mapYesNoBool(el('f_android') || 'No'),
          provide_tizen: mapYesNoBool(el('f_tizen') || 'No'),
          provide_webos: mapYesNoBool(el('f_webos') || 'No'),
          setup_appium_servers: mapYesNoBool(el('f_appium') || 'No'),
          wda_bundle_id: el('f_wda') || '',
          supervision_password: el('f_sup') || '',
        }
        if (!body.nickname || !body.host_address) {
          toast('Nickname, host and port are required')
          return
        }
        try {
          if (idx == null) await addProvider(body)
          else await updateProvider(body)
          setModal(null)
          await load()
          toast(idx == null ? `${body.nickname} added` : `${body.nickname} updated`)
        } catch (e) {
          toast(e instanceof Error ? e.message : 'Save failed')
        }
      },
    })
  }

  return (
    <>
      <div className="abar">
        <button type="button" className="addb" style={{ margin: 0 }} onClick={() => openForm(null)}>
          <i className="ti ti-plus" />
          Add provider
        </button>
      </div>
      <table className="tbl">
        <thead>
          <tr>
            <th>Provider</th>
            <th>OS</th>
            <th>Host</th>
            <th>Provides</th>
            <th>WDA bundle</th>
            <th>Devices</th>
            <th>Status</th>
            <th style={{ width: 120 }} />
          </tr>
        </thead>
        <tbody>
          {providers.map((p, i) => (
            <tr key={p.nickname}>
              <td style={{ fontWeight: 600 }}>{p.nickname}</td>
              <td>{mapOsFromBackend(p.os)}</td>
              <td className="mono">
                {p.host_address}:{p.port}
              </td>
              <td>
                {[
                  p.provide_ios && 'iOS',
                  p.provide_android && 'Android',
                  p.provide_tizen && 'Tizen',
                  p.provide_webos && 'WebOS',
                ]
                  .filter(Boolean)
                  .join(', ') || '—'}
              </td>
              <td className="mono" style={{ fontSize: 11 }}>
                {p.wda_bundle_id || '—'}
              </td>
              <td>{deviceCounts[p.nickname] || 0}</td>
              <td>
                <span className={`stat ${p.state === 'live' ? 'ok' : 'bad'}`}>
                  {p.state === 'live' ? 'live' : 'down'}
                </span>
              </td>
              <td style={{ textAlign: 'right' }}>
                <button
                  type="button"
                  className="ib"
                  title="Show logs"
                  onClick={() => toast(`Showing logs for ${p.nickname}`)}
                >
                  <i className="ti ti-file-text" />
                </button>
                <button
                  type="button"
                  className="ib"
                  title="Restart"
                  onClick={() =>
                    toast(`Restart ${p.nickname}: no hub endpoint — restart process manually`)
                  }
                >
                  <i className="ti ti-refresh" />
                </button>
                <button type="button" className="ib" title="Update provider" onClick={() => openForm(i)}>
                  <i className="ti ti-edit" />
                </button>
                <button
                  type="button"
                  className="ib del"
                  title="Delete provider"
                  onClick={async () => {
                    try {
                      await deleteProvider(p.nickname)
                      await load()
                      toast(`Removed ${p.nickname}`)
                    } catch (e) {
                      toast(e instanceof Error ? e.message : 'Delete failed')
                    }
                  }}
                >
                  <i className="ti ti-trash" />
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="fnote">
        All updates to existing provider config require provider instance restart.
      </div>
    </>
  )
}

function ProviderFormFields({ st }: { st: Record<string, string> }) {
  return (
    <>
      <FormRow label="OS *">
        <select className="fi2" id="f_pos" defaultValue={st.os}>
          {['Windows', 'macOS', 'Linux'].map((o) => (
            <option key={o}>{o}</option>
          ))}
        </select>
      </FormRow>
      <FormRow label="Nickname *">
        <input className="fi2" id="f_nick" defaultValue={st.nickname} />
      </FormRow>
      <FormRow label="Host address *">
        <input className="fi2" id="f_host" defaultValue={st.host} />
      </FormRow>
      <FormRow label="Port *">
        <input className="fi2" id="f_port" type="number" defaultValue={st.port} />
      </FormRow>
      <div className="fgrid">
        {(['f_ios', 'f_android', 'f_tizen', 'f_webos'] as const).map((id, i) => (
          <FormRow key={id} label={`Provide ${['iOS', 'Android', 'Tizen', 'WebOS'][i]}? *`}>
            <select className="fi2" id={id} defaultValue={st[id.replace('f_', '') as keyof typeof st] || 'No'}>
              <option>No</option>
              <option>Yes</option>
            </select>
          </FormRow>
        ))}
      </div>
      <FormRow label="Setup Appium servers? *">
        <select className="fi2" id="f_appium" defaultValue={st.appium}>
          <option>No</option>
          <option>Yes</option>
        </select>
      </FormRow>
      <FormRow label="WDA bundle ID">
        <input className="fi2" id="f_wda" defaultValue={st.wda} />
      </FormRow>
      <FormRow label="iOS supervision profile password">
        <input className="fi2" id="f_sup" type="password" defaultValue={st.sup} />
      </FormRow>
    </>
  )
}

function DevicesTab({
  toast,
  setModal,
}: {
  toast: (m: string, i?: string) => void
  setModal: ModalSetter
}) {
  const [devices, setDevices] = useState<DBDevice[]>([])
  const [providers, setProviders] = useState<Provider[]>([])
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [streamTypes, setStreamTypes] = useState<string[]>([])

  const load = useCallback(async () => {
    const [d, ws] = await Promise.all([getAdminDevices(), getAdminWorkspaces()])
    setDevices(d.devices)
    setProviders(d.providers)
    setStreamTypes(d.device_stream_types)
    setWorkspaces(ws)
  }, [])

  useEffect(() => {
    load().catch(() => toast('Failed to load devices'))
  }, [load, toast])

  const openForm = (idx: number | null) => {
    const d = idx == null ? ({} as DBDevice) : devices[idx]
    setModal({
      title: idx == null ? 'Add device' : 'Update device',
      note: 'All updates to existing devices require respective provider restart.',
      ok: idx == null ? 'Add device' : 'Update device',
      body: <DeviceFormFields d={d} providers={providers} workspaces={workspaces} streamTypes={streamTypes} />,
      onSubmit: async () => {
        const el = (id: string) => (document.getElementById(id) as HTMLInputElement)?.value.trim()
        const body: DBDevice = {
          udid: el('f_udid') || '',
          os: mapOsToBackend(el('f_os') || 'iOS'),
          name: el('f_name') || '',
          os_version: el('f_ver') || '',
          screen_width: el('f_sw') || '1080',
          screen_height: el('f_sh') || '2400',
          usage: (el('f_usage') || 'Enabled').toLowerCase(),
          provider: el('f_prov') || '',
          stream_type: el('f_stream') || 'MJPEG',
          device_type: (el('f_type') || 'Real device').includes('Emulator') ? 'emulator' : 'real',
          workspace_id:
            workspaces.find((w) => w.name === el('f_ws'))?.id || el('f_ws') || '',
        }
        if (!body.udid || !body.name || !body.os_version) {
          toast('UDID, name and OS version are required')
          return
        }
        try {
          if (idx == null) await addDevice(body)
          else await updateDevice(body)
          setModal(null)
          await load()
          toast(idx == null ? `${body.name} added` : `${body.name} updated`)
        } catch (e) {
          toast(e instanceof Error ? e.message : 'Save failed')
        }
      },
    })
  }

  return (
    <>
      <div className="abar">
        <button type="button" className="addb" style={{ margin: 0 }} onClick={() => openForm(null)}>
          <i className="ti ti-plus" />
          Add device
        </button>
      </div>
      <table className="tbl">
        <thead>
          <tr>
            <th>Device</th>
            <th>OS</th>
            <th>UDID</th>
            <th>Usage</th>
            <th>Stream</th>
            <th>Provider</th>
            <th>Workspace</th>
            <th>Status</th>
            <th style={{ width: 90 }} />
          </tr>
        </thead>
        <tbody>
          {devices.map((d, i) => (
            <tr key={d.udid}>
              <td style={{ fontWeight: 600 }}>{d.name}</td>
              <td>
                {mapOsFromBackend(d.os)} {d.os_version}
              </td>
              <td className="mono" style={{ fontSize: 11 }}>
                {d.udid}
              </td>
              <td>{d.usage}</td>
              <td style={{ fontSize: 12 }}>{d.stream_type}</td>
              <td>{d.provider}</td>
              <td>{workspaces.find((w) => w.id === d.workspace_id)?.name || d.workspace_id}</td>
              <td>
                <span className="stat ok">online</span>
              </td>
              <td style={{ textAlign: 'right' }}>
                <button
                  type="button"
                  className="ib"
                  title="Re-provision device"
                  onClick={() => toast(`Re-provisioning ${d.name}: no hub endpoint`)}
                >
                  <i className="ti ti-rotate-clockwise" />
                </button>
                <button type="button" className="ib" title="Update device" onClick={() => openForm(i)}>
                  <i className="ti ti-edit" />
                </button>
                <button
                  type="button"
                  className="ib del"
                  title="Delete device"
                  onClick={async () => {
                    try {
                      await deleteDevice(d.udid)
                      await load()
                      toast(`Removed ${d.name}`)
                    } catch (e) {
                      toast(e instanceof Error ? e.message : 'Delete failed')
                    }
                  }}
                >
                  <i className="ti ti-trash" />
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="fnote">All updates to existing devices require respective provider restart.</div>
    </>
  )
}

function DeviceFormFields({
  d,
  providers,
  workspaces,
  streamTypes,
}: {
  d: DBDevice
  providers: Provider[]
  workspaces: Workspace[]
  streamTypes: string[]
}) {
  const streams =
    streamTypes.length > 0
      ? streamTypes
      : ['WebRTC - Broadcast Extension', 'WebRTC - FFMpeg', 'MJPEG']
  return (
    <>
      <FormRow label="Device OS *">
        <select className="fi2" id="f_os" defaultValue={mapOsFromBackend(d.os || 'ios')}>
          {['iOS', 'Android', 'Tizen', 'WebOS'].map((o) => (
            <option key={o}>{o}</option>
          ))}
        </select>
      </FormRow>
      <FormRow label="Device type *">
        <select className="fi2" id="f_type" defaultValue={d.device_type === 'emulator' ? 'Emulator/Simulator' : 'Real device'}>
          <option>Real device</option>
          <option>Emulator/Simulator</option>
        </select>
      </FormRow>
      <FormRow label="UDID *">
        <input className="fi2" id="f_udid" defaultValue={d.udid || ''} />
      </FormRow>
      <FormRow label="Name *">
        <input className="fi2" id="f_name" defaultValue={d.name || ''} />
      </FormRow>
      <FormRow label="OS version *">
        <input className="fi2" id="f_ver" defaultValue={d.os_version || ''} />
      </FormRow>
      <div className="fgrid">
        <FormRow label="Screen width">
          <input className="fi2" id="f_sw" type="number" defaultValue={d.screen_width || '1080'} />
        </FormRow>
        <FormRow label="Screen height">
          <input className="fi2" id="f_sh" type="number" defaultValue={d.screen_height || '2400'} />
        </FormRow>
      </div>
      <FormRow label="Device usage *">
        <select className="fi2" id="f_usage" defaultValue={d.usage ? d.usage.charAt(0).toUpperCase() + d.usage.slice(1) : 'Enabled'}>
          <option>Enabled</option>
          <option>Disabled</option>
        </select>
      </FormRow>
      <FormRow label="Provider *">
        <select className="fi2" id="f_prov" defaultValue={d.provider || providers[0]?.nickname}>
          {providers.map((p) => (
            <option key={p.nickname}>{p.nickname}</option>
          ))}
        </select>
      </FormRow>
      <FormRow label="Video stream type *">
        <select className="fi2" id="f_stream" defaultValue={d.stream_type || streams[0]}>
          {streams.map((s) => (
            <option key={s}>{s}</option>
          ))}
        </select>
      </FormRow>
      <FormRow label="Workspace *">
        <select className="fi2" id="f_ws" defaultValue={workspaces.find((w) => w.id === d.workspace_id)?.name || workspaces[0]?.name}>
          {workspaces.map((w) => (
            <option key={w.id}>{w.name}</option>
          ))}
        </select>
      </FormRow>
    </>
  )
}

function UsersTab({
  toast,
  setModal,
}: {
  toast: (m: string, i?: string) => void
  setModal: ModalSetter
}) {
  const [users, setUsers] = useState<{ username: string; role: string; workspace_ids?: string[] }[]>([])
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])

  const load = useCallback(async () => {
    const [u, ws] = await Promise.all([getUsers(), getAdminWorkspaces()])
    setUsers(u)
    setWorkspaces(ws)
  }, [])

  useEffect(() => {
    load().catch(() => toast('Failed to load users'))
  }, [load, toast])

  const openForm = (idx: number | null) => {
    const u = idx == null ? null : users[idx]
    const isAdmin = u?.username === 'admin'
    setModal({
      title: idx == null ? 'Add user' : 'Update user',
      ok: idx == null ? 'Add user' : 'Update user',
      body: (
        <>
          <FormRow label="Username *">
            <input
              className="fi2"
              id="f_un"
              defaultValue={u?.username || ''}
              readOnly={!!u}
              style={u ? { opacity: 0.6 } : undefined}
            />
          </FormRow>
          <FormRow label={idx == null ? 'Password *' : 'Password (leave empty to keep)'}>
            <input className="fi2" id="f_pw" type="password" />
          </FormRow>
          <FormRow label="User role *">
            <select className="fi2" id="f_role" defaultValue={u?.role === 'admin' ? 'Admin' : 'User'}>
              <option>User</option>
              <option>Admin</option>
            </select>
          </FormRow>
          {!isAdmin && (
            <FormRow label="Workspaces *">
              <select className="fi2" id="f_uws" defaultValue={workspaces.find((w) => w.id === u?.workspace_ids?.[0])?.name || workspaces[0]?.name}>
                {workspaces.map((w) => (
                  <option key={w.id}>{w.name}</option>
                ))}
              </select>
            </FormRow>
          )}
        </>
      ),
      onSubmit: async () => {
        const el = (id: string) => (document.getElementById(id) as HTMLInputElement)?.value.trim()
        const wsName = el('f_uws')
        const wsId = workspaces.find((w) => w.name === wsName)?.id
        const body = {
          username: el('f_un') || '',
          password: el('f_pw') || '',
          role: (el('f_role') || 'User').toLowerCase(),
          workspace_ids: wsId ? [wsId] : [],
        }
        if (!body.username || (idx == null && !body.password)) {
          toast('Username and password are required')
          return
        }
        try {
          if (idx == null) await addUser(body)
          else await updateUser(body)
          setModal(null)
          await load()
          toast(`${body.username} ${idx == null ? 'added' : 'updated'}`)
        } catch (e) {
          toast(e instanceof Error ? e.message : 'Save failed')
        }
      },
    })
  }

  return (
    <>
      <div className="abar">
        <button type="button" className="addb" style={{ margin: 0 }} onClick={() => openForm(null)}>
          <i className="ti ti-plus" />
          Add user
        </button>
      </div>
      <table className="tbl">
        <thead>
          <tr>
            <th>Username</th>
            <th>Role</th>
            <th>Workspaces</th>
            <th style={{ width: 120 }} />
          </tr>
        </thead>
        <tbody>
          {users.map((u, i) => (
            <tr key={u.username}>
              <td style={{ fontWeight: 600 }}>{u.username}</td>
              <td>{u.role}</td>
              <td>
                {u.username === 'admin'
                  ? 'All'
                  : u.workspace_ids
                      ?.map((id) => workspaces.find((w) => w.id === id)?.name || id)
                      .join(', ') || '—'}
              </td>
              <td style={{ textAlign: 'right' }}>
                <button type="button" className="ib" title="Update user" onClick={() => openForm(i)}>
                  <i className="ti ti-edit" />
                </button>
                {u.username !== 'admin' && (
                  <button
                    type="button"
                    className="ib del"
                    title="Delete user"
                    onClick={async () => {
                      try {
                        await deleteUser(u.username)
                        await load()
                        toast(`Deleted ${u.username}`)
                      } catch (e) {
                        toast(e instanceof Error ? e.message : 'Delete failed')
                      }
                    }}
                  >
                    <i className="ti ti-trash" />
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="fnote">
        The default admin user cannot be deleted — only its password can change.
      </div>
    </>
  )
}

function FilesTab({ toast }: { toast: (m: string, i?: string) => void }) {
  const [status, setStatus] = useState<{ selenium?: boolean; supervision?: boolean; wda?: boolean }>({})

  const load = useCallback(async () => {
    const f = await getAdminFiles()
    setStatus({
      selenium: f.selenium,
      supervision: f.supervision,
      wda: f.webdriveragent,
    })
  }, [])

  useEffect(() => {
    load().catch(() => {})
  }, [load])

  const upload = async (type: 'selenium' | 'supervision' | 'wda', file: File) => {
    try {
      if (type === 'selenium') await uploadSeleniumJar(file)
      else if (type === 'supervision') await uploadSupervision(file)
      else await uploadWDA(file)
      await load()
      toast(`${file.name} uploaded`, 'ti-upload')
    } catch (e) {
      toast(
        e instanceof Error
          ? e.message
          : type === 'selenium'
            ? 'Selenium jar upload not supported by hub'
            : 'Upload failed',
      )
    }
  }

  return (
    <div className="upwrap">
      <UploadCard
        title="Upload Selenium jar"
        text="If you want to connect provider Appium nodes to Selenium Grid instance you need to upload a valid Selenium jar. Version 4.13 is recommended."
        exists={!!status.selenium}
        accept=".jar"
        onUpload={(f) => void upload('selenium', f)}
      />
      <UploadCard
        title="Upload supervision profile"
        text="Upload the supervision profile if you are using supervised iOS devices."
        exists={!!status.supervision}
        accept=".p12,.mobileconfig"
        onUpload={(f) => void upload('supervision', f)}
      />
      <UploadCard
        title="Upload WebDriverAgent IPA"
        text="Upload signed WebDriverAgent IPA file"
        exists={!!status.wda}
        accept=".ipa"
        onUpload={(f) => void upload('wda', f)}
      />
    </div>
  )
}

function UploadCard({
  title,
  text,
  exists,
  accept,
  onUpload,
}: {
  title: string
  text: string
  exists: boolean
  accept: string
  onUpload: (f: File) => void
}) {
  return (
    <div className="upc">
      <h4>{title}</h4>
      <p>{text}</p>
      <button
        type="button"
        className="ab"
        style={{ flex: 'none', padding: '8px 13px' }}
        onClick={() => {
          const input = document.createElement('input')
          input.type = 'file'
          input.accept = accept
          input.onchange = () => {
            const f = input.files?.[0]
            if (f) onUpload(f)
          }
          input.click()
        }}
      >
        <i className="ti ti-upload" />
        Select and upload
      </button>
      <div className={`upst ${exists ? 'ok' : ''}`}>
        {exists ? 'File exists.' : 'No uploaded file.'}
      </div>
    </div>
  )
}

function SettingsTab({ toast }: { toast: (m: string, i?: string) => void }) {
  const [stream, setStream] = useState<StreamSettings | null>(null)
  const [turn, setTurn] = useState<TURNConfig | null>(null)
  const [turnEnabled, setTurnEnabled] = useState(false)

  useEffect(() => {
    Promise.all([getGlobalSettings(), getTurnConfig(), getSystemStatus()])
      .then(([s, t]) => {
        if (s) setStream(s)
        if (t) {
          setTurn(t)
          setTurnEnabled(t.enabled)
        }
      })
      .catch(() => {})
  }, [])

  const fpsOpts = ['5', '10', '15', '30']
  const scaleOpts = ['25', '50', '75', '100']

  return (
    <div style={{ display: 'flex', gap: 14, flexWrap: 'wrap', alignItems: 'flex-start' }}>
      <div className="card" style={{ width: 320, margin: 0 }}>
        <h3>
          <i className="ti ti-video" />
          Stream Settings
        </h3>
        <FormRow label="Target FPS">
          <select className="fi2" id="g_fps" defaultValue={String(stream?.target_fps || 15)}>
            {fpsOpts.map((f) => (
              <option key={f} value={f}>
                {f} FPS
              </option>
            ))}
          </select>
        </FormRow>
        <FormRow label="JPEG Quality">
          <select className="fi2" id="g_jpeg" defaultValue={String(stream?.jpeg_quality || 75)}>
            {['50', '75', '90'].map((q) => (
              <option key={q} value={q}>
                {q}
              </option>
            ))}
          </select>
        </FormRow>
        <FormRow label="Scaling Factor Android">
          <select
            className="fi2"
            id="g_sca"
            defaultValue={String(stream?.scaling_factor_android || 50)}
          >
            {scaleOpts.map((s) => (
              <option key={s} value={s}>
                {s}%
              </option>
            ))}
          </select>
        </FormRow>
        <FormRow label="Scaling Factor iOS">
          <select className="fi2" id="g_sci" defaultValue={String(stream?.scaling_factor_ios || 50)}>
            {scaleOpts.map((s) => (
              <option key={s} value={s}>
                {s}%
              </option>
            ))}
          </select>
        </FormRow>
        <button
          type="button"
          className="fsub"
          style={{ width: '100%' }}
          onClick={async () => {
            const el = (id: string) => (document.getElementById(id) as HTMLSelectElement)?.value
            const body: StreamSettings = {
              target_fps: parseInt(el('g_fps') || '15', 10),
              jpeg_quality: parseInt(el('g_jpeg') || '75', 10),
              scaling_factor_android: parseInt(el('g_sca') || '50', 10),
              scaling_factor_ios: parseInt(el('g_sci') || '50', 10),
            }
            try {
              await saveGlobalSettings(body)
              toast('Stream settings saved')
            } catch (e) {
              toast(e instanceof Error ? e.message : 'Save failed')
            }
          }}
        >
          Save Settings
        </button>
      </div>
      <div className="card" style={{ width: 360, margin: 0 }}>
        <h3>
          <i className="ti ti-shield-cog" />
          TURN Server Configuration
        </h3>
        <div className="infob" style={{ marginBottom: 10 }}>
          Required for WebRTC behind NAT/firewalls. Uses secure ephemeral credentials.
        </div>
        <div className="infob" style={{ marginBottom: 10 }}>
          Security: Auto-expiring credentials. Generate secret:{' '}
          <code>openssl rand -base64 32</code>
        </div>
        <div className="krow">
          <span className="k">Enable TURN Server</span>
          <button
            type="button"
            className={`tog ${turnEnabled ? 'on' : ''}`}
            onClick={() => setTurnEnabled((v) => !v)}
          />
        </div>
        <FormRow label="TURN Server">
          <input className="fi2" id="g_turn" defaultValue={turn?.server || ''} placeholder="hostname or IP" />
          <div className="fnote">TURN server hostname or IP address</div>
        </FormRow>
        <FormRow label="Port">
          <input className="fi2" id="g_tport" type="number" defaultValue={String(turn?.port || 3478)} />
          <div className="fnote">TURN server port (default: 3478)</div>
        </FormRow>
        <FormRow label="Shared Secret">
          <input className="fi2" id="g_tsec" type="password" defaultValue={turn?.shared_secret || ''} />
          <div className="fnote">Secret for credential generation</div>
        </FormRow>
        <FormRow label="TTL seconds">
          <input className="fi2" id="g_ttl" type="number" defaultValue={String(turn?.ttl || 3600)} />
          <div className="fnote">Credential lifetime (default: 3600s/1h)</div>
        </FormRow>
        <button
          type="button"
          className="fsub"
          style={{ width: '100%' }}
          onClick={async () => {
            const el = (id: string) => (document.getElementById(id) as HTMLInputElement)?.value
            const body: TURNConfig = {
              enabled: turnEnabled,
              server: el('g_turn') || '',
              port: parseInt(el('g_tport') || '3478', 10),
              shared_secret: el('g_tsec') || '',
              ttl: parseInt(el('g_ttl') || '3600', 10),
            }
            try {
              await saveTurnConfig(body)
              toast('TURN config saved — applies to new WebRTC connections')
            } catch (e) {
              toast(e instanceof Error ? e.message : 'Save failed')
            }
          }}
        >
          Save TURN Config
        </button>
        <div className="fnote" style={{ marginTop: 10 }}>
          Changes apply immediately to new WebRTC connections.
        </div>
        <div className="fnote">
          Setup help:{' '}
          <a
            href="https://github.com/shamanec/GADS/blob/master/docs/webrtc-turn-server-guide.md"
            target="_blank"
            rel="noreferrer"
          >
            See TURN Deployment Guide
          </a>
        </div>
      </div>
    </div>
  )
}

function WorkspacesTab({
  toast,
  setModal,
}: {
  toast: (m: string, i?: string) => void
  setModal: ModalSetter
}) {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [search, setSearch] = useState('')

  const load = useCallback(async () => {
    setWorkspaces(await getAdminWorkspaces())
  }, [])

  useEffect(() => {
    load().catch(() => toast('Failed to load workspaces'))
  }, [load, toast])

  const filtered = workspaces.filter(
    (w) => !search || w.name.toLowerCase().includes(search.toLowerCase()),
  )

  const openForm = (idx: number | null) => {
    const w = idx == null ? null : workspaces[idx]
    setModal({
      title: idx == null ? 'Add workspace' : 'Edit workspace',
      ok: idx == null ? 'Add workspace' : 'Save',
      body: (
        <>
          <FormRow label="Workspace name *">
            <input className="fi2" id="f_wn" defaultValue={w?.name || ''} />
          </FormRow>
          <FormRow label="Description">
            <input className="fi2" id="f_wd" defaultValue={w?.description || ''} />
          </FormRow>
        </>
      ),
      onSubmit: async () => {
        const name = (document.getElementById('f_wn') as HTMLInputElement)?.value.trim()
        const description = (document.getElementById('f_wd') as HTMLInputElement)?.value.trim()
        if (!name) {
          toast('Name is required')
          return
        }
        try {
          if (idx == null) await addWorkspace({ name, description })
          else await updateWorkspace({ ...workspaces[idx], name, description })
          setModal(null)
          await load()
          toast(`Workspace ${name} saved`)
        } catch (e) {
          toast(e instanceof Error ? e.message : 'Save failed')
        }
      },
    })
  }

  return (
    <>
      <div className="abar">
        <div className="search">
          <i className="ti ti-search" />
          <input
            placeholder="Search workspaces"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <button type="button" className="addb" style={{ margin: 0 }} onClick={() => openForm(null)}>
          <i className="ti ti-plus" />
          Add Workspace
        </button>
      </div>
      <table className="tbl">
        <thead>
          <tr>
            <th>Workspace name</th>
            <th>Description</th>
            <th>Tenant</th>
            <th>Type</th>
            <th>Devices</th>
            <th style={{ width: 90 }} />
          </tr>
        </thead>
        <tbody>
          {filtered.map((w, i) => (
            <tr key={w.id}>
              <td style={{ fontWeight: 600 }}>{w.name}</td>
              <td>{w.description || ''}</td>
              <td className="mono" style={{ fontSize: 11 }}>
                {w.tenant_id || '—'}{' '}
                <button
                  type="button"
                  className="ib"
                  style={{ width: 20, height: 20 }}
                  title="Copy tenant"
                  onClick={() => {
                    void navigator.clipboard.writeText(w.tenant_id || '')
                    toast('Tenant ID copied')
                  }}
                >
                  <i className="ti ti-copy" style={{ fontSize: 12 }} />
                </button>
              </td>
              <td>{w.is_default ? 'Default' : 'Custom'}</td>
              <td>{w.device_count ?? 0}</td>
              <td style={{ textAlign: 'right' }}>
                <button type="button" className="ib" title="Edit" onClick={() => openForm(i)}>
                  <i className="ti ti-edit" />
                </button>
                {!w.is_default && (
                  <button
                    type="button"
                    className="ib del"
                    title="Delete"
                    onClick={async () => {
                      try {
                        await deleteWorkspace(w.id)
                        await load()
                        toast(`Deleted workspace ${w.name}`)
                      } catch (e) {
                        toast(e instanceof Error ? e.message : 'Delete failed')
                      }
                    }}
                  >
                    <i className="ti ti-trash" />
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  )
}

function KeysTab({
  toast,
  setModal,
}: {
  toast: (m: string, i?: string) => void
  setModal: ModalSetter
}) {
  const [keys, setKeys] = useState<SecretKey[]>([])
  const [filter, setFilter] = useState('')

  const load = useCallback(async () => {
    setKeys(await getSecretKeys())
  }, [])

  useEffect(() => {
    load().catch(() => toast('Failed to load secret keys'))
  }, [load, toast])

  const filtered = keys.filter(
    (k) => !filter || k.origin.toLowerCase().includes(filter.toLowerCase()),
  )

  const openForm = (idx: number | null) => {
    const k = idx == null ? null : keys[idx]
    setModal({
      title: idx == null ? 'Add secret key' : 'Edit secret key',
      note: 'Secret keys let external JWT issuers authenticate against the hub.',
      ok: 'Save',
      body: (
        <>
          <FormRow label="Origin *">
            <input className="fi2" id="f_ko" defaultValue={k?.origin || ''} />
          </FormRow>
          <FormRow label="User identifier claim *">
            <input className="fi2" id="f_kc" defaultValue={k?.user_identifier_claim || 'username'} />
          </FormRow>
          <FormRow label="Tenant identifier claim">
            <input
              className="fi2"
              id="f_kt"
              defaultValue={k?.tenant_identifier_claim === '-' ? '' : k?.tenant_identifier_claim || ''}
            />
          </FormRow>
        </>
      ),
      onSubmit: async () => {
        const origin = (document.getElementById('f_ko') as HTMLInputElement)?.value.trim()
        const user_identifier_claim = (document.getElementById('f_kc') as HTMLInputElement)?.value.trim()
        const tenant_identifier_claim =
          (document.getElementById('f_kt') as HTMLInputElement)?.value.trim() || '-'
        if (!origin || !user_identifier_claim) {
          toast('Origin and user claim are required')
          return
        }
        try {
          if (idx == null)
            await addSecretKey({ origin, user_identifier_claim, tenant_identifier_claim })
          else await updateSecretKey(keys[idx].id, { origin, user_identifier_claim, tenant_identifier_claim })
          setModal(null)
          await load()
          toast(`Secret key ${origin} saved`)
        } catch (e) {
          toast(e instanceof Error ? e.message : 'Save failed')
        }
      },
    })
  }

  return (
    <>
      <div className="abar">
        <div className="search">
          <i className="ti ti-search" />
          <input
            placeholder="Filter by origin"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>
        <button type="button" className="addb" style={{ margin: 0 }} onClick={() => openForm(null)}>
          <i className="ti ti-plus" />
          Add Secret Key
        </button>
        <button
          type="button"
          className="ab"
          style={{ flex: 'none', padding: '8px 13px' }}
          onClick={async () => {
            try {
              await getSecretKeyHistory()
              toast('Key history loaded')
            } catch {
              toast('Key history — empty or failed')
            }
          }}
        >
          <i className="ti ti-history" />
          View History
        </button>
      </div>
      <table className="tbl">
        <thead>
          <tr>
            <th>Origin</th>
            <th>User identifier claim</th>
            <th>Tenant identifier claim</th>
            <th>Status</th>
            <th>Created at</th>
            <th>Updated at</th>
            <th style={{ width: 110 }} />
          </tr>
        </thead>
        <tbody>
          {filtered.map((k, i) => (
            <tr key={k.id}>
              <td style={{ fontWeight: 600 }}>{k.origin}</td>
              <td>{k.user_identifier_claim}</td>
              <td>{k.tenant_identifier_claim}</td>
              <td>
                <span className={`stat ${k.enabled ? 'ok' : 'bad'}`}>{k.status}</span>
              </td>
              <td>{k.created_at}</td>
              <td>{k.updated_at}</td>
              <td style={{ textAlign: 'right' }}>
                <button type="button" className="ib" title="Edit" onClick={() => openForm(i)}>
                  <i className="ti ti-edit" />
                </button>
                <button
                  type="button"
                  className="ab"
                  style={{
                    flex: 'none',
                    padding: '4px 10px',
                    fontSize: '11.5px',
                    color: 'var(--warn)',
                    borderColor: '#f0d9b0',
                  }}
                  onClick={async () => {
                    try {
                      await disableSecretKey(k.id)
                      await load()
                      toast(`${k.origin} ${k.enabled ? 'disabled' : 'enabled'}`)
                    } catch (e) {
                      toast(e instanceof Error ? e.message : 'Toggle failed')
                    }
                  }}
                >
                  {k.enabled ? 'Disable' : 'Enable'}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  )
}

function CredsTab({
  toast,
  setModal,
}: {
  toast: (m: string, i?: string) => void
  setModal: ModalSetter
}) {
  const [creds, setCreds] = useState<ClientCredential[]>([])
  const [filter, setFilter] = useState('')
  const [newSecret, setNewSecret] = useState<ClientCredential | null>(null)
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])

  const load = useCallback(async () => {
    const [c, ws] = await Promise.all([getClientCredentials(), getAdminWorkspaces()])
    setCreds(c)
    setWorkspaces(ws)
  }, [])

  useEffect(() => {
    load().catch(() => toast('Failed to load credentials'))
  }, [load, toast])

  const filtered = creds.filter(
    (c) =>
      !filter ||
      c.name.toLowerCase().includes(filter.toLowerCase()) ||
      c.client_id.toLowerCase().includes(filter.toLowerCase()) ||
      (c.description || '').toLowerCase().includes(filter.toLowerCase()),
  )

  const openCreate = () => {
    setModal({
      title: 'Create new credential',
      note: 'The client ID and secret are generated once — copy them after creation.',
      ok: 'Create',
      body: (
        <>
          <FormRow label="Name *">
            <input className="fi2" id="f_cn" />
          </FormRow>
          <FormRow label="Description">
            <input className="fi2" id="f_cd" />
          </FormRow>
          <FormRow label="Workspace *">
            <select className="fi2" id="f_cw" defaultValue={workspaces[0]?.name}>
              {workspaces.map((w) => (
                <option key={w.id}>{w.name}</option>
              ))}
            </select>
          </FormRow>
        </>
      ),
      onSubmit: async () => {
        const name = (document.getElementById('f_cn') as HTMLInputElement)?.value.trim()
        const description = (document.getElementById('f_cd') as HTMLInputElement)?.value.trim()
        if (!name) {
          toast('Name is required')
          return
        }
        try {
          const created = await createClientCredential({ name, description })
          setModal(null)
          setNewSecret(created)
          await load()
          toast('Credential created — secret shown once, copy it')
        } catch (e) {
          toast(e instanceof Error ? e.message : 'Create failed')
        }
      },
    })
  }

  return (
    <>
      {newSecret && (
        <div className="secret-once">
          <strong>Client ID:</strong>{' '}
          <span className="mono">{newSecret.client_id}</span>{' '}
          <button
            type="button"
            className="ib"
            onClick={() => {
              void navigator.clipboard.writeText(newSecret.client_id)
              toast('Client ID copied')
            }}
          >
            <i className="ti ti-copy" />
          </button>
          {newSecret.client_secret && (
            <>
              <br />
              <strong>Client secret (shown once):</strong>{' '}
              <span className="mono">{newSecret.client_secret}</span>{' '}
              <button
                type="button"
                className="ib"
                onClick={() => {
                  void navigator.clipboard.writeText(newSecret.client_secret!)
                  toast('Secret copied')
                }}
              >
                <i className="ti ti-copy" />
              </button>
            </>
          )}
          <button
            type="button"
            className="fcan"
            style={{ marginTop: 8 }}
            onClick={() => setNewSecret(null)}
          >
            Dismiss
          </button>
        </div>
      )}
      <div className="abar">
        <div className="search">
          <i className="ti ti-search" />
          <input
            placeholder="Filter by name, description or client ID"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>
        <button type="button" className="addb" style={{ margin: 0 }} onClick={openCreate}>
          <i className="ti ti-plus" />
          Create New Credential
        </button>
        <button
          type="button"
          className="ab"
          style={{ flex: 'none', padding: '8px 13px' }}
          onClick={() =>
            setModal({
              title: 'Capabilities example',
              ok: 'Close',
              body: (
                <pre
                  style={{
                    background: 'var(--bg)',
                    border: '1px solid var(--bd)',
                    borderRadius: 8,
                    padding: 12,
                    fontSize: 12,
                    overflowX: 'auto',
                  }}
                >
                  {`{
  "platformName": "iOS",
  "appium:automationName": "XCUITest",
  "appium:platformVersion": "17.6.1",
  "appium:udid": "auto",
  "gads:credentialId": "cc_xxxxx"
}`}
                </pre>
              ),
              onSubmit: () => setModal(null),
            })
          }
        >
          <i className="ti ti-info-circle" />
          View Capabilities Example
        </button>
      </div>
      {filtered.length === 0 ? (
        <div
          className="empty"
          style={{
            border: '1px solid var(--bd)',
            borderRadius: 12,
            background: 'var(--panel)',
          }}
        >
          No client credentials found. Add your first credential using the button above.
        </div>
      ) : (
        <table className="tbl">
          <thead>
            <tr>
              <th>Name</th>
              <th>Description</th>
              <th>Client ID</th>
              <th>Workspace</th>
              <th>Created</th>
              <th style={{ width: 60 }} />
            </tr>
          </thead>
          <tbody>
            {filtered.map((c) => (
              <tr key={c.id}>
                <td style={{ fontWeight: 600 }}>{c.name}</td>
                <td>{c.description || ''}</td>
                <td className="mono">
                  {c.client_id}{' '}
                  <button
                    type="button"
                    className="ib"
                    style={{ width: 20, height: 20 }}
                    title="Copy"
                    onClick={() => {
                      void navigator.clipboard.writeText(c.client_id)
                      toast('Client ID copied')
                    }}
                  >
                    <i className="ti ti-copy" style={{ fontSize: 12 }} />
                  </button>
                </td>
                <td>{workspaces.find((w) => w.id === c.workspace_id)?.name || '—'}</td>
                <td>{c.created_at || '—'}</td>
                <td style={{ textAlign: 'right' }}>
                  <button
                    type="button"
                    className="ib del"
                    title="Delete"
                    onClick={async () => {
                      try {
                        await deleteClientCredential(c.id)
                        await load()
                        toast('Credential deleted')
                      } catch (e) {
                        toast(e instanceof Error ? e.message : 'Delete failed')
                      }
                    }}
                  >
                    <i className="ti ti-trash" />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}

function ActionsTab({
  toast,
  setModal,
}: {
  toast: (m: string, i?: string) => void
  setModal: ModalSetter
}) {
  const [actions, setActions] = useState<CustomAction[]>([])

  const load = useCallback(async () => {
    setActions(await getCustomActions())
  }, [])

  useEffect(() => {
    load().catch(() => toast('Failed to load actions'))
  }, [load, toast])

  const favCount = actions.filter((a) => a.is_favorite).length

  const toggleFav = async (a: CustomAction) => {
    if (!a.is_favorite && favCount >= 5) {
      toast('Max 5 favorites')
      return
    }
    try {
      if (a.is_favorite) await removeFavoriteAction(a.id)
      else await addFavoriteAction(a.id)
      await load()
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Favorite toggle failed')
    }
  }

  const openForm = (idx: number | null) => {
    const a = idx == null ? null : actions[idx]
    setModal({
      title: idx == null ? 'New Action' : 'Edit Action',
      note: 'Up to 5 actions can be favorites — favorites appear as buttons in the device panel.',
      ok: 'Save',
      body: (
        <>
          <FormRow label="Name *">
            <input className="fi2" id="f_an" defaultValue={a?.name || ''} />
          </FormRow>
          <FormRow label="Description">
            <input className="fi2" id="f_ad" defaultValue={a?.description || ''} />
          </FormRow>
          <FormRow label="Action type *">
            <select className="fi2" id="f_at" defaultValue={a?.action_type || 'Tap'}>
              {['Tap', 'Swipe', 'Text input', 'Key press'].map((t) => (
                <option key={t}>{t}</option>
              ))}
            </select>
          </FormRow>
          <FormRow label="Value (coordinates / text / key)">
            <input
              className="fi2"
              id="f_av"
              defaultValue={(a?.parameters as { value?: string })?.value || ''}
            />
          </FormRow>
        </>
      ),
      onSubmit: async () => {
        const name = (document.getElementById('f_an') as HTMLInputElement)?.value.trim()
        const description = (document.getElementById('f_ad') as HTMLInputElement)?.value.trim()
        const action_type = (document.getElementById('f_at') as HTMLSelectElement)?.value
        const value = (document.getElementById('f_av') as HTMLInputElement)?.value.trim()
        if (!name) {
          toast('Name is required')
          return
        }
        const body = {
          name,
          description,
          action_type: action_type.toLowerCase().replace(' ', '_'),
          parameters: { value },
        }
        try {
          if (idx == null) await addCustomAction(body)
          else await updateCustomAction(actions[idx].id, body)
          setModal(null)
          await load()
          toast(`Action ${name} saved`)
        } catch (e) {
          toast(e instanceof Error ? e.message : 'Save failed')
        }
      },
    })
  }

  return (
    <>
      <div className="infob">
        Custom Actions allow you to create reusable automation tasks (tap, swipe, text input, etc.)
        that can be executed on devices. Actions can be executed from the device control panel. You
        can mark up to 5 actions as favorites for quick access - favorites appear as buttons in the
        device panel, while other actions are available via dropdown. Only admins can create, edit,
        and delete custom actions.
      </div>
      <div className="abar">
        <button type="button" className="addb" style={{ margin: 0 }} onClick={() => openForm(null)}>
          <i className="ti ti-plus" />
          New Action
        </button>
      </div>
      {actions.length === 0 ? (
        <div
          className="empty"
          style={{
            border: '1px solid var(--bd)',
            borderRadius: 12,
            background: 'var(--panel)',
          }}
        >
          No actions available. Click &quot;New Action&quot; to create one.
        </div>
      ) : (
        <table className="tbl">
          <thead>
            <tr>
              <th style={{ width: 36 }} />
              <th>Name</th>
              <th>Description</th>
              <th>Type</th>
              <th>Value</th>
              <th style={{ width: 90 }} />
            </tr>
          </thead>
          <tbody>
            {actions.map((a, i) => (
              <tr key={a.id}>
                <td>
                  <button
                    type="button"
                    className="ib"
                    title="Favorite"
                    onClick={() => void toggleFav(a)}
                  >
                    <i className={`ti ti-star star ${a.is_favorite ? 'on' : ''}`} />
                  </button>
                </td>
                <td style={{ fontWeight: 600 }}>{a.name}</td>
                <td>{a.description || ''}</td>
                <td>{a.action_type}</td>
                <td className="mono" style={{ fontSize: 11 }}>
                  {(a.parameters as { value?: string })?.value || '—'}
                </td>
                <td style={{ textAlign: 'right' }}>
                  <button type="button" className="ib" title="Edit" onClick={() => openForm(i)}>
                    <i className="ti ti-edit" />
                  </button>
                  <button
                    type="button"
                    className="ib del"
                    title="Delete"
                    onClick={async () => {
                      try {
                        await deleteCustomAction(a.id)
                        await load()
                        toast('Action deleted')
                      } catch (e) {
                        toast(e instanceof Error ? e.message : 'Delete failed')
                      }
                    }}
                  >
                    <i className="ti ti-trash" />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}
