import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

interface AdminGateModalProps {
  open: boolean
  targetTab: string
  onCancel: () => void
}

export function AdminGateModal({ open, targetTab, onCancel }: AdminGateModalProps) {
  const { verifyPassword } = useAuth()
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  if (!open) return null

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!password.trim()) {
      setError('Admin password required')
      return
    }
    setLoading(true)
    setError('')
    const ok = await verifyPassword(password)
    setLoading(false)
    if (ok) {
      setPassword('')
      navigate(`/admin/${targetTab}`)
    } else {
      setError('Incorrect password')
    }
  }

  const handleCancel = () => {
    setPassword('')
    setError('')
    onCancel()
  }

  return (
    <div className="gate-overlay">
      <form className="lcard gate-card" onSubmit={handleSubmit}>
        <div className="llogo gate-card">
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
        <button type="button" className="gate-cancel" onClick={handleCancel}>
          Cancel
        </button>
      </form>
    </div>
  )
}
