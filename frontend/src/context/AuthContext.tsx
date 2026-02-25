import { createContext, useContext, useState, useEffect, useCallback, useRef } from 'react'

const SESSION_TIMEOUT_KEY = 'liftoff-session-timeout-minutes'
const DEFAULT_SESSION_TIMEOUT = 15

export function getSessionTimeoutMinutes(): number {
  const stored = localStorage.getItem(SESSION_TIMEOUT_KEY)
  const parsed = parseInt(stored || '', 10)
  return Number.isFinite(parsed) && parsed >= 1 ? parsed : DEFAULT_SESSION_TIMEOUT
}

export function setSessionTimeoutMinutes(minutes: number): void {
  localStorage.setItem(SESSION_TIMEOUT_KEY, String(Math.max(1, minutes)))
}

export interface User {
  id: string
  email: string
  isAdmin?: boolean
}

export interface AuthState {
  user: User | null
  token: string | null
  expiresAt: string | null
  isLoading: boolean
  isAuthenticated: boolean
  isAdmin: boolean
}

interface AuthContextType extends AuthState {
  login: (email: string, password: string, rememberMe?: boolean) => Promise<void>
  register: (email: string, password: string) => Promise<void>
  logout: () => void
  sessionTimeoutMinutes: number
  setSessionTimeoutMinutes: (minutes: number) => void
  showAdmin: boolean
  setShowAdmin: (show: boolean) => void
}

const AUTH_KEY = 'liftoff-auth'
// Use relative URL - Vite proxies /api to backend in dev
const API_BASE = '/api'

interface StoredAuth {
  token: string
  user: User
  expiresAt: string
}

function getStoredAuth(): StoredAuth | null {
  try {
    const stored = localStorage.getItem(AUTH_KEY)
    if (!stored) return null
    const data = JSON.parse(stored) as StoredAuth
    if (data.expiresAt && new Date(data.expiresAt) < new Date()) {
      localStorage.removeItem(AUTH_KEY)
      return null
    }
    return data
  } catch {
    return null
  }
}

function storeAuth(data: StoredAuth | null) {
  if (data) {
    localStorage.setItem(AUTH_KEY, JSON.stringify(data))
  } else {
    localStorage.removeItem(AUTH_KEY)
  }
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [expiresAt, setExpiresAt] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [sessionTimeoutMinutes, setSessionTimeoutState] = useState(getSessionTimeoutMinutes)
  const [showAdmin, setShowAdmin] = useState(false)
  const idleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const applyAuth = useCallback((data: StoredAuth | null) => {
    if (data) {
      setToken(data.token)
      setUser(data.user)
      setExpiresAt(data.expiresAt)
    } else {
      setToken(null)
      setUser(null)
      setExpiresAt(null)
    }
  }, [])

  useEffect(() => {
    const stored = getStoredAuth()
    if (stored) {
      applyAuth(stored)
      // Refresh user from /auth/me to get isAdmin (and sync any backend changes)
      fetch(`${API_BASE}/auth/me`, {
        headers: { Authorization: `Bearer ${stored.token}` },
      })
        .then((res) => (res.ok ? res.json() : null))
        .then((data) => {
          if (data?.user) {
            const updated = { ...stored, user: { ...stored.user, ...data.user } }
            storeAuth(updated)
            applyAuth(updated)
          }
        })
        .catch(() => {})
    }
    setIsLoading(false)
  }, [applyAuth])

  // When API gets 401, sync auth state (logout)
  useEffect(() => {
    const handler = () => {
      storeAuth(null)
      applyAuth(null)
    }
    window.addEventListener('liftoff:unauthorized', handler)
    return () => window.removeEventListener('liftoff:unauthorized', handler)
  }, [applyAuth])

  const login = useCallback(async (email: string, password: string, rememberMe = false) => {
    const res = await fetch(`${API_BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password, rememberMe }),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      throw new Error(err.error || 'Login failed')
    }
    const data = await res.json()
    const authData: StoredAuth = {
      token: data.token,
      user: data.user,
      expiresAt: data.expiresAt,
    }
    storeAuth(authData)
    applyAuth(authData)
  }, [applyAuth])

  const register = useCallback(async (email: string, password: string) => {
    const res = await fetch(`${API_BASE}/auth/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      throw new Error(err.error || 'Registration failed')
    }
    const data = await res.json()
    const authData: StoredAuth = {
      token: data.token,
      user: data.user,
      expiresAt: data.expiresAt,
    }
    storeAuth(authData)
    applyAuth(authData)
  }, [applyAuth])

  const logout = useCallback(() => {
    if (idleTimerRef.current) {
      clearTimeout(idleTimerRef.current)
      idleTimerRef.current = null
    }
    storeAuth(null)
    applyAuth(null)
  }, [applyAuth])

  // Idle timeout: log out after N minutes of no activity (only when not "remember me" - short token)
  useEffect(() => {
    if (!token || !user) return
    const stored = getStoredAuth()
    if (!stored) return
    const expiry = new Date(stored.expiresAt)
    const isLongLived = expiry.getTime() - Date.now() > 24 * 60 * 60 * 1000 // > 1 day = remember me
    if (isLongLived) return // Don't apply idle timeout for remember-me sessions

    const resetIdleTimer = () => {
      if (idleTimerRef.current) clearTimeout(idleTimerRef.current)
      idleTimerRef.current = setTimeout(() => {
        storeAuth(null)
        applyAuth(null)
        idleTimerRef.current = null
      }, sessionTimeoutMinutes * 60 * 1000)
    }

    resetIdleTimer()
    const events = ['mousedown', 'keydown', 'scroll', 'touchstart']
    events.forEach((e) => window.addEventListener(e, resetIdleTimer))
    return () => {
      if (idleTimerRef.current) clearTimeout(idleTimerRef.current)
      events.forEach((e) => window.removeEventListener(e, resetIdleTimer))
    }
  }, [token, user, sessionTimeoutMinutes, applyAuth])

  const updateSessionTimeout = useCallback((minutes: number) => {
    const value = Math.max(1, Math.min(120, minutes))
    setSessionTimeoutState(value)
    setSessionTimeoutMinutes(value)
  }, [])

  const value: AuthContextType = {
    user,
    token,
    expiresAt,
    isLoading,
    isAuthenticated: !!token,
    isAdmin: !!user?.isAdmin,
    login,
    register,
    logout,
    sessionTimeoutMinutes,
    setSessionTimeoutMinutes: updateSessionTimeout,
    showAdmin,
    setShowAdmin,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}

export function getAuthToken(): string | null {
  const stored = getStoredAuth()
  return stored?.token ?? null
}
