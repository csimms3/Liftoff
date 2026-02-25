import { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { apiService, type AdminUser, type AdminStats } from '../api'
import './AdminPanel.css'

interface AdminPanelProps {
  onBack: () => void
}

export function AdminPanel({ onBack }: AdminPanelProps) {
  const { user, logout } = useAuth()
  const [users, setUsers] = useState<AdminUser[]>([])
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const [usersData, statsData] = await Promise.all([
          apiService.getAdminUsers(),
          apiService.getAdminStats(),
        ])
        if (!cancelled) {
          setUsers(usersData)
          setStats(statsData)
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load admin data')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => { cancelled = true }
  }, [])

  if (loading) {
    return (
      <div className="admin-panel">
        <header className="admin-header">
          <button type="button" className="admin-back" onClick={onBack}>
            Back to app
          </button>
        </header>
        <div className="admin-loading">Loading admin data...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="admin-panel">
        <header className="admin-header">
          <button type="button" className="admin-back" onClick={onBack}>
            Back to app
          </button>
        </header>
        <div className="admin-error">{error}</div>
      </div>
    )
  }

  return (
    <div className="admin-panel">
      <header className="admin-header">
        <div>
          <h1 className="admin-title">Admin Panel</h1>
          <p className="admin-subtitle">Signed in as {user?.email}</p>
        </div>
        <div className="admin-header-actions">
          <button type="button" className="admin-back" onClick={onBack}>
            Back to app
          </button>
          <button type="button" className="admin-logout" onClick={logout}>
            Sign out
          </button>
        </div>
      </header>

      <main className="admin-main">
        {stats && (
          <section className="admin-stats">
            <h2>Summary</h2>
            <div className="admin-stats-grid">
              <div className="admin-stat-card">
                <span className="admin-stat-value">{stats.total_users}</span>
                <span className="admin-stat-label">Total users</span>
              </div>
              <div className="admin-stat-card">
                <span className="admin-stat-value">{stats.new_users_7d}</span>
                <span className="admin-stat-label">New users (7 days)</span>
              </div>
              <div className="admin-stat-card">
                <span className="admin-stat-value">{stats.total_workouts}</span>
                <span className="admin-stat-label">Total workouts</span>
              </div>
              <div className="admin-stat-card">
                <span className="admin-stat-value">{stats.total_sessions}</span>
                <span className="admin-stat-label">Total sessions</span>
              </div>
            </div>
          </section>
        )}

        <section className="admin-users">
          <h2>Registered accounts ({users.length})</h2>
          <div className="admin-users-table-wrap">
            <table className="admin-users-table">
              <thead>
                <tr>
                  <th>Email</th>
                  <th>User ID</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => (
                  <tr key={u.id}>
                    <td>{u.email}</td>
                    <td className="admin-user-id">{u.id}</td>
                    <td>{new Date(u.created_at).toLocaleDateString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </main>
    </div>
  )
}
