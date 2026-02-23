import { useState } from 'react'
import './AuthPages.css'

const API_BASE = 'http://localhost:8080/api'

const PASSWORD_RULES = 'At least 8 characters, one number, one capital letter, and one special character'

interface ResetPasswordPageProps {
  token: string
  onSuccess: () => void
}

export function ResetPasswordPage({ token, onSuccess }: ResetPasswordPageProps) {
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [success, setSuccess] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }
    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }
    if (!/[0-9]/.test(password)) {
      setError('Password must contain at least one number')
      return
    }
    if (!/[A-Z]/.test(password)) {
      setError('Password must contain at least one capital letter')
      return
    }
    if (!/[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>/?~`]/.test(password)) {
      setError('Password must contain at least one special character')
      return
    }

    setLoading(true)
    try {
      const res = await fetch(`${API_BASE}/auth/reset-password`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, newPassword: password }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        throw new Error(data.error || 'Reset failed')
      }
      setSuccess(true)
      setTimeout(onSuccess, 2000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Reset failed')
    } finally {
      setLoading(false)
    }
  }

  if (success) {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <h1 className="auth-title">Password reset!</h1>
          <p className="auth-subtitle">Redirecting you to sign in...</p>
        </div>
      </div>
    )
  }

  return (
    <div className="auth-page">
      <div className="auth-card">
        <h1 className="auth-title">Reset password</h1>
        <p className="auth-subtitle">Enter your new password</p>

        <form onSubmit={handleSubmit} className="auth-form">
          {error && <div className="auth-error">{error}</div>}
          <div className="auth-field">
            <label htmlFor="password">New password</label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              required
              autoComplete="new-password"
              title={PASSWORD_RULES}
            />
            <span className="auth-hint">{PASSWORD_RULES}</span>
          </div>
          <div className="auth-field">
            <label htmlFor="confirmPassword">Confirm password</label>
            <input
              id="confirmPassword"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="••••••••"
              required
              autoComplete="new-password"
            />
          </div>
          <button type="submit" className="auth-button" disabled={loading}>
            {loading ? 'Resetting...' : 'Reset password'}
          </button>
        </form>
      </div>
    </div>
  )
}
