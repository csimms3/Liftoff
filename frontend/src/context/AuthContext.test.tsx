import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AuthProvider, useAuth, getSessionTimeoutMinutes, setSessionTimeoutMinutes } from './AuthContext'

function TestConsumer() {
  const { sessionTimeoutMinutes, setSessionTimeoutMinutes: set } = useAuth()
  return (
    <div>
      <span data-testid="timeout">{sessionTimeoutMinutes}</span>
      <button onClick={() => set(30)}>Set 30</button>
    </div>
  )
}

describe('AuthContext', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  it('getSessionTimeoutMinutes returns default when not set', () => {
    localStorage.clear()
    expect(getSessionTimeoutMinutes()).toBe(15)
  })

  it('setSessionTimeoutMinutes persists value', () => {
    setSessionTimeoutMinutes(30)
    expect(getSessionTimeoutMinutes()).toBe(30)
  })

  it('provides session timeout to consumers', async () => {
    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    )
    await waitFor(() => {
      expect(screen.getByTestId('timeout')).toHaveTextContent('15')
    })
  })

  it('updates session timeout when set', async () => {
    const user = userEvent.setup()
    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    )
    await user.click(screen.getByText('Set 30'))
    await waitFor(() => {
      expect(screen.getByTestId('timeout')).toHaveTextContent('30')
    })
  })
})
