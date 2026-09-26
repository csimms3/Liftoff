import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import App from './App'
import { AuthProvider } from './context/AuthContext'

// Mock fetch for API calls
const mockFetch = vi.fn()
beforeEach(() => {
  vi.stubGlobal('fetch', mockFetch)
  localStorage.clear()
  // Pre-populate auth so user is logged in
  localStorage.setItem('liftoff-auth', JSON.stringify({
    token: 'test-token',
    user: { id: '1', email: 'test@test.com' },
    expiresAt: new Date(Date.now() + 86400000).toISOString(),
  }))
})
afterEach(() => {
  vi.unstubAllGlobals()
})

function renderWithAuth(ui: React.ReactElement) {
  return render(
    <AuthProvider>
      {ui}
    </AuthProvider>
  )
}

describe('App', () => {
  beforeEach(() => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/workouts')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve([]) })
      }
      return Promise.resolve({ ok: false })
    })
  })

  test('renders app title', async () => {
    renderWithAuth(<App />)
    await waitFor(() => {
      const headings = screen.getAllByRole('heading', { name: /liftoff/i })
      expect(headings.length).toBeGreaterThan(0)
    })
  })

  test('shows create workout form', async () => {
    renderWithAuth(<App />)
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Workout name...')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create/i })).toBeInTheDocument()
    })
  })

  test('shows loading state initially', async () => {
    renderWithAuth(<App />)
    await waitFor(() => {
      expect(screen.getByText('Loading workouts...')).toBeInTheDocument()
    })
  })
})

describe('App — workout in progress', () => {
  const workout = { id: 'w1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
  const active = { id: 's1', workout_id: 'w1', workout, started_at: '', is_active: true, exercises: [] }

  beforeEach(() => {
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      const json = (body: unknown) => Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body) })
      if (url.includes('/sessions/active')) return json(active)
      if (url.includes('/sessions') && init?.method === 'POST') return json(active)
      if (url.endsWith('/workouts')) return json([workout])
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
    })
  })

  test('"Continue Session" shows the session without starting (and so ending) one', async () => {
    renderWithAuth(<App />)
    const button = await screen.findByRole('button', { name: 'Continue Session' })
    fireEvent.click(button)
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Continue Session' })).toBeNull())
    const started = mockFetch.mock.calls.filter(([url, init]) => String(url).endsWith('/sessions') && init?.method === 'POST')
    expect(started).toHaveLength(0)
  })
})
