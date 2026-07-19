import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { AdminPage } from './AdminPage'

const TABS = [
  'providers',
  'devices',
  'users',
  'files',
  'settings',
  'workspaces',
  'keys',
  'creds',
  'actions',
] as const

export function AdminRoute() {
  const { tab } = useParams()
  const navigate = useNavigate()
  const { verifyPassword } = useAuth()
  const [unlocked, setUnlocked] = useState(false)
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const activeTab = TABS.includes(tab as (typeof TABS)[number]) ? tab! : 'providers'

  if (!unlocked) {
    return (
      <div className="gate-overlay">
        <form
          className="lcard gate-card"
          onSubmit={async (e) => {
            e.preventDefault()
            if (!password.trim()) {
              setError('Admin password required')
              return
            }
            setLoading(true)
            setError('')
            const ok = await verifyPassword(password)
            setLoading(false)
            if (ok) setUnlocked(true)
            else setError('Incorrect password')
          }}
        >
          <div className="llogo" style={{ background: 'var(--acc)' }}>
            <i className="ti ti-shield-lock" />
          </div>
          <h1>Admin access</h1>
          <p className="sub2">Enter the admin password — asked every time</p>
          {error && <div className="lerr">{error}</div>}
          <input
            className="fi"
            type="password"
            placeholder="Admin password"
            autoComplete="off"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoFocus
          />
          <button className="lbtn" type="submit" disabled={loading}>
            {loading ? 'Unlocking…' : 'Unlock'}
          </button>
          <button
            type="button"
            className="gate-cancel"
            onClick={() => navigate('/')}
          >
            Cancel
          </button>
        </form>
      </div>
    )
  }

  return <AdminPage tab={activeTab} />
}
