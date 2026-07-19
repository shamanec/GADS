import { NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { useToast } from './ToastContext'

interface SidebarProps {
  active: 'devices' | 'providers' | 'admin' | 'settings' | 'none'
}

export function Sidebar({ active }: SidebarProps) {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const { toast } = useToast()

  const handleLogout = async () => {
    await logout()
    toast('Signed out', 'ti-logout')
    navigate('/login')
  }

  const goAdmin = (tab: string) => navigate(`/admin/${tab}`)

  return (
    <aside className="side">
      <div className="logo">
        <i className="ti ti-device-mobile" />
      </div>
      <NavLink
        to="/"
        className={`snav ${active === 'devices' ? 'on' : ''}`}
        title="Devices"
      >
        <i className="ti ti-layout-grid" />
      </NavLink>
      <button
        type="button"
        className={`snav ${active === 'providers' ? 'on' : ''}`}
        title="Providers"
        onClick={() => goAdmin('providers')}
      >
        <i className="ti ti-server" />
      </button>
      <button
        type="button"
        className={`snav ${active === 'admin' ? 'on' : ''}`}
        title="Admin"
        onClick={() => goAdmin('users')}
      >
        <i className="ti ti-users" />
      </button>
      <div className="side-spacer" />
      <button
        type="button"
        className={`snav ${active === 'settings' ? 'on' : ''}`}
        title="Settings"
        onClick={() => goAdmin('settings')}
      >
        <i className="ti ti-settings" />
      </button>
      <button type="button" className="snav" title="Sign out" onClick={handleLogout}>
        <i className="ti ti-logout" />
      </button>
    </aside>
  )
}
