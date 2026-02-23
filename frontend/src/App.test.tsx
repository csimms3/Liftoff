import { render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
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
    expect(screen.getByText('Create')).toBeInTheDocument()
  })

  test('shows loading state initially', async () => {
    renderWithAuth(<App />)
    await waitFor(() => {
      expect(screen.getByText('Loading workouts...')).toBeInTheDocument()
    })
  })
})
