import { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { LoginPage } from './LoginPage'
import { RegisterPage } from './RegisterPage'
import { ForgotPasswordPage } from './ForgotPasswordPage'
import { ResetPasswordPage } from './ResetPasswordPage'
import { AdminPanel } from './AdminPanel'
import App from '../App'

function getResetToken(): string | null {
  const params = new URLSearchParams(window.location.search)
  return params.get('token')
}

export function AuthGate() {
  const { isAuthenticated, isLoading, isAdmin, showAdmin, setShowAdmin } = useAuth()
  const [showRegister, setShowRegister] = useState(false)
  const [showForgotPassword, setShowForgotPassword] = useState(false)
  const [resetToken, setResetToken] = useState<string | null>(null)

  useEffect(() => {
    const token = getResetToken()
    if (token) {
      setResetToken(token)
      window.history.replaceState({}, '', window.location.pathname)
    }
  }, [])

  if (resetToken) {
    return (
      <ResetPasswordPage
        token={resetToken}
        onSuccess={() => setResetToken(null)}
      />
    )
  }

  if (isLoading) {
    return (
      <div
        style={{
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: 'var(--bg-primary)',
        }}
      >
        <span style={{ color: 'var(--text-secondary)' }}>Loading...</span>
      </div>
    )
  }

  if (!isAuthenticated) {
    if (showForgotPassword) {
      return (
        <ForgotPasswordPage
          onSwitchToLogin={() => setShowForgotPassword(false)}
        />
      )
    }
    return showRegister ? (
      <RegisterPage onSwitchToLogin={() => setShowRegister(false)} />
    ) : (
      <LoginPage
        onSwitchToRegister={() => setShowRegister(true)}
        onSwitchToForgotPassword={() => setShowForgotPassword(true)}
        onSwitchToAdmin={(show) => setShowAdmin(show)}
        isAdminLogin={showAdmin}
      />
    )
  }

  if (showAdmin) {
    if (!isAdmin) {
      return (
        <div
          style={{
            minHeight: '100vh',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '1rem',
            background: 'var(--bg-primary)',
          }}
        >
          <p style={{ color: 'var(--text-primary)' }}>Access denied. Admin only.</p>
          <button
            type="button"
            onClick={() => setShowAdmin(false)}
            style={{
              padding: '0.5rem 1rem',
              background: 'var(--bg-secondary)',
              border: '1px solid var(--border-color)',
              borderRadius: '8px',
              cursor: 'pointer',
            }}
          >
            Back to app
          </button>
        </div>
      )
    }
    return <AdminPanel onBack={() => setShowAdmin(false)} />
  }

  return <App />
}
