import { useState } from 'react'
import { useAuth } from '../context/AuthContext'
import { LoginPage } from './LoginPage'
import { RegisterPage } from './RegisterPage'
import App from '../App'

export function AuthGate() {
  const { isAuthenticated, isLoading } = useAuth()
  const [showRegister, setShowRegister] = useState(false)

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
    return showRegister ? (
      <RegisterPage onSwitchToLogin={() => setShowRegister(false)} />
    ) : (
      <LoginPage onSwitchToRegister={() => setShowRegister(true)} />
    )
  }

  return <App />
}
