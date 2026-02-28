import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AuthProvider, useAuth, getSessionTimeoutMinutes, setSessionTimeoutMinutes } from './AuthContext'

// Helper: consumer component that exposes full auth state for assertions
function AuthConsumer() {
  const { user, isAuthenticated, isAdmin, login, logout, register } = useAuth()
  return (
    <div>
      <span data-testid="email">{user?.email ?? 'none'}</span>
      <span data-testid="authenticated">{isAuthenticated ? 'yes' : 'no'}</span>
      <span data-testid="admin">{isAdmin ? 'yes' : 'no'}</span>
      <button onClick={() => login('user@example.com', 'Pass1!', false)}>Login</button>
      <button onClick={() => login('admin@liftoff.local', 'Admin123!', false)}>Login Admin</button>
      <button onClick={() => register('new@example.com', 'Pass1!')}>Register</button>
      <button onClick={logout}>Logout</button>
    </div>
  )
}

function SessionConsumer() {
  const { sessionTimeoutMinutes, setSessionTimeoutMinutes: set } = useAuth()
  return (
    <div>
      <span data-testid="timeout">{sessionTimeoutMinutes}</span>
      <button onClick={() => set(30)}>Set 30</button>
    </div>
  )
}

function mockFetchSuccess(body: object) {
  return vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve(body),
  })
}

function mockFetchFailure(status: number, error: string) {
  return vi.fn().mockResolvedValue({
    ok: false,
    status,
    json: () => Promise.resolve({ error }),
  })
}

const fakeAuthResponse = {
  token: 'fake-jwt-token',
  expiresAt: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
  user: { id: 'u1', email: 'user@example.com', isAdmin: false },
}

const fakeAdminAuthResponse = {
  token: 'fake-admin-token',
  expiresAt: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
  user: { id: 'admin', email: 'admin@liftoff.local', isAdmin: true },
}

describe('AuthContext — session timeout', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })
  afterEach(() => vi.unstubAllGlobals())

  it('getSessionTimeoutMinutes returns default when not set', () => {
    expect(getSessionTimeoutMinutes()).toBe(15)
  })

  it('setSessionTimeoutMinutes persists value', () => {
    setSessionTimeoutMinutes(30)
    expect(getSessionTimeoutMinutes()).toBe(30)
  })

  it('provides session timeout to consumers', async () => {
    render(<AuthProvider><SessionConsumer /></AuthProvider>)
    await waitFor(() => expect(screen.getByTestId('timeout')).toHaveTextContent('15'))
  })

  it('updates session timeout when set', async () => {
    const user = userEvent.setup()
    render(<AuthProvider><SessionConsumer /></AuthProvider>)
    await user.click(screen.getByText('Set 30'))
    await waitFor(() => expect(screen.getByTestId('timeout')).toHaveTextContent('30'))
  })
})

describe('AuthContext — login', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => vi.unstubAllGlobals())

  it('successful login sets user and isAuthenticated', async () => {
    vi.stubGlobal('fetch', mockFetchSuccess(fakeAuthResponse))
    const user = userEvent.setup()
    render(<AuthProvider><AuthConsumer /></AuthProvider>)
    await user.click(screen.getByText('Login'))
    await waitFor(() => {
      expect(screen.getByTestId('authenticated')).toHaveTextContent('yes')
      expect(screen.getByTestId('email')).toHaveTextContent('user@example.com')
    })
  })

  it('successful login persists token to localStorage', async () => {
    vi.stubGlobal('fetch', mockFetchSuccess(fakeAuthResponse))
    const user = userEvent.setup()
    render(<AuthProvider><AuthConsumer /></AuthProvider>)
    await user.click(screen.getByText('Login'))
    await waitFor(() => {
      const stored = localStorage.getItem('liftoff-auth')
      expect(stored).not.toBeNull()
      const parsed = JSON.parse(stored!)
      expect(parsed.token).toBe('fake-jwt-token')
    })
  })

  it('failed login throws and does not authenticate', async () => {
    vi.stubGlobal('fetch', mockFetchFailure(401, 'invalid email or password'))
    const user = userEvent.setup()

    // Use a component that captures thrown errors
    let caughtError: string | null = null
    function ErrorCapture() {
      const { login, isAuthenticated } = useAuth()
      return (
        <div>
          <span data-testid="authenticated">{isAuthenticated ? 'yes' : 'no'}</span>
          <button onClick={() => login('bad@bad.com', 'Wrong1!').catch(e => { caughtError = e.message })}>
            Bad Login
          </button>
        </div>
      )
    }
    render(<AuthProvider><ErrorCapture /></AuthProvider>)
    await user.click(screen.getByText('Bad Login'))
    await waitFor(() => expect(caughtError).toContain('invalid email or password'))
    expect(screen.getByTestId('authenticated')).toHaveTextContent('no')
  })
})

describe('AuthContext — register', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => vi.unstubAllGlobals())

  it('successful register sets user and isAuthenticated', async () => {
    vi.stubGlobal('fetch', mockFetchSuccess(fakeAuthResponse))
    const user = userEvent.setup()
    render(<AuthProvider><AuthConsumer /></AuthProvider>)
    await user.click(screen.getByText('Register'))
    await waitFor(() => {
      expect(screen.getByTestId('authenticated')).toHaveTextContent('yes')
    })
  })

  it('failed register throws and does not authenticate', async () => {
    vi.stubGlobal('fetch', mockFetchFailure(400, 'email already in use'))
    let caughtError: string | null = null
    function ErrorCapture() {
      const { register, isAuthenticated } = useAuth()
      return (
        <div>
          <span data-testid="authenticated">{isAuthenticated ? 'yes' : 'no'}</span>
          <button onClick={() => register('dup@example.com', 'Pass1!').catch(e => { caughtError = e.message })}>
            Bad Register
          </button>
        </div>
      )
    }
    const user = userEvent.setup()
    render(<AuthProvider><ErrorCapture /></AuthProvider>)
    await user.click(screen.getByText('Bad Register'))
    await waitFor(() => expect(caughtError).toContain('email already in use'))
    expect(screen.getByTestId('authenticated')).toHaveTextContent('no')
  })
})

describe('AuthContext — logout', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => vi.unstubAllGlobals())

  it('logout clears user and isAuthenticated', async () => {
    vi.stubGlobal('fetch', mockFetchSuccess(fakeAuthResponse))
    const user = userEvent.setup()
    render(<AuthProvider><AuthConsumer /></AuthProvider>)
    await user.click(screen.getByText('Login'))
    await waitFor(() => expect(screen.getByTestId('authenticated')).toHaveTextContent('yes'))
    await user.click(screen.getByText('Logout'))
    await waitFor(() => expect(screen.getByTestId('authenticated')).toHaveTextContent('no'))
    expect(screen.getByTestId('email')).toHaveTextContent('none')
  })

  it('logout removes token from localStorage', async () => {
    vi.stubGlobal('fetch', mockFetchSuccess(fakeAuthResponse))
    const user = userEvent.setup()
    render(<AuthProvider><AuthConsumer /></AuthProvider>)
    await user.click(screen.getByText('Login'))
    await waitFor(() => expect(localStorage.getItem('liftoff-auth')).not.toBeNull())
    await user.click(screen.getByText('Logout'))
    await waitFor(() => expect(localStorage.getItem('liftoff-auth')).toBeNull())
  })
})

describe('AuthContext — isAdmin', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => vi.unstubAllGlobals())

  it('isAdmin is false for regular users', async () => {
    vi.stubGlobal('fetch', mockFetchSuccess(fakeAuthResponse))
    const user = userEvent.setup()
    render(<AuthProvider><AuthConsumer /></AuthProvider>)
    await user.click(screen.getByText('Login'))
    await waitFor(() => expect(screen.getByTestId('admin')).toHaveTextContent('no'))
  })

  it('isAdmin is true when backend returns isAdmin: true', async () => {
    vi.stubGlobal('fetch', mockFetchSuccess(fakeAdminAuthResponse))
    const user = userEvent.setup()
    render(<AuthProvider><AuthConsumer /></AuthProvider>)
    await user.click(screen.getByText('Login Admin'))
    await waitFor(() => expect(screen.getByTestId('admin')).toHaveTextContent('yes'))
  })

  it('isAdmin is true for admin@liftoff.local even without backend flag', async () => {
    const responseWithoutFlag = {
      ...fakeAdminAuthResponse,
      user: { id: 'admin', email: 'admin@liftoff.local', isAdmin: false },
    }
    vi.stubGlobal('fetch', mockFetchSuccess(responseWithoutFlag))
    const user = userEvent.setup()
    render(<AuthProvider><AuthConsumer /></AuthProvider>)
    await user.click(screen.getByText('Login Admin'))
    await waitFor(() => expect(screen.getByTestId('admin')).toHaveTextContent('yes'))
  })
})

describe('AuthContext — 401 auto-logout', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => vi.unstubAllGlobals())

  it('liftoff:unauthorized event logs the user out', async () => {
    vi.stubGlobal('fetch', mockFetchSuccess(fakeAuthResponse))
    const user = userEvent.setup()
    render(<AuthProvider><AuthConsumer /></AuthProvider>)
    await user.click(screen.getByText('Login'))
    await waitFor(() => expect(screen.getByTestId('authenticated')).toHaveTextContent('yes'))

    act(() => {
      window.dispatchEvent(new Event('liftoff:unauthorized'))
    })

    await waitFor(() => expect(screen.getByTestId('authenticated')).toHaveTextContent('no'))
    expect(localStorage.getItem('liftoff-auth')).toBeNull()
  })
})
