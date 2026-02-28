import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AuthProvider } from '../context/AuthContext'
import { RegisterPage } from './RegisterPage'

function renderRegister(onSwitch = vi.fn()) {
  return render(
    <AuthProvider>
      <RegisterPage onSwitchToLogin={onSwitch} />
    </AuthProvider>
  )
}

describe('RegisterPage', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })
  afterEach(() => vi.unstubAllGlobals())

  it('renders all form fields and submit button', () => {
    renderRegister()
    expect(screen.getByRole('heading', { name: /liftoff/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/^email/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^password$/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/confirm password/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /create account/i })).toBeInTheDocument()
  })

  it('renders sign in link', () => {
    renderRegister()
    expect(screen.getByText(/sign in/i)).toBeInTheDocument()
  })

  it('calls onSwitchToLogin when sign in is clicked', async () => {
    const onSwitch = vi.fn()
    const user = userEvent.setup()
    renderRegister(onSwitch)
    await user.click(screen.getByText(/sign in/i))
    expect(onSwitch).toHaveBeenCalled()
  })

  it('shows error when passwords do not match', async () => {
    const user = userEvent.setup()
    renderRegister()
    await user.type(screen.getByLabelText(/^email/i), 'test@example.com')
    await user.type(screen.getByLabelText(/^password$/i), 'Password1!')
    await user.type(screen.getByLabelText(/confirm password/i), 'Different1!')
    await user.click(screen.getByRole('button', { name: /create account/i }))
    await waitFor(() => expect(screen.getByText(/passwords do not match/i)).toBeInTheDocument())
  })

  it('shows error for password shorter than 8 characters', async () => {
    const user = userEvent.setup()
    renderRegister()
    await user.type(screen.getByLabelText(/^email/i), 'test@example.com')
    await user.type(screen.getByLabelText(/^password$/i), 'Sh0!')
    await user.type(screen.getByLabelText(/confirm password/i), 'Sh0!')
    await user.click(screen.getByRole('button', { name: /create account/i }))
    await waitFor(() => expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument())
  })

  it('shows error when password has no number', async () => {
    const user = userEvent.setup()
    renderRegister()
    await user.type(screen.getByLabelText(/^email/i), 'test@example.com')
    await user.type(screen.getByLabelText(/^password$/i), 'Password!')
    await user.type(screen.getByLabelText(/confirm password/i), 'Password!')
    await user.click(screen.getByRole('button', { name: /create account/i }))
    await waitFor(() => expect(screen.getByText(/at least one number/i)).toBeInTheDocument())
  })

  it('shows error when password has no capital letter', async () => {
    const user = userEvent.setup()
    renderRegister()
    await user.type(screen.getByLabelText(/^email/i), 'test@example.com')
    await user.type(screen.getByLabelText(/^password$/i), 'password1!')
    await user.type(screen.getByLabelText(/confirm password/i), 'password1!')
    await user.click(screen.getByRole('button', { name: /create account/i }))
    await waitFor(() => expect(screen.getByText('Password must contain at least one capital letter')).toBeInTheDocument())
  })

  it('shows error when password has no special character', async () => {
    const user = userEvent.setup()
    renderRegister()
    await user.type(screen.getByLabelText(/^email/i), 'test@example.com')
    await user.type(screen.getByLabelText(/^password$/i), 'Password1')
    await user.type(screen.getByLabelText(/confirm password/i), 'Password1')
    await user.click(screen.getByRole('button', { name: /create account/i }))
    await waitFor(() => expect(screen.getByText('Password must contain at least one special character')).toBeInTheDocument())
  })

  it('shows API error on failed registration', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      json: () => Promise.resolve({ error: 'email already in use' }),
    }))
    const user = userEvent.setup()
    renderRegister()
    await user.type(screen.getByLabelText(/^email/i), 'taken@example.com')
    await user.type(screen.getByLabelText(/^password$/i), 'Password1!')
    await user.type(screen.getByLabelText(/confirm password/i), 'Password1!')
    await user.click(screen.getByRole('button', { name: /create account/i }))
    await waitFor(() => expect(screen.getByText(/email already in use/i)).toBeInTheDocument())
  })

  it('disables button and shows loading text while submitting', async () => {
    let resolve: () => void
    vi.stubGlobal('fetch', vi.fn().mockReturnValue(
      new Promise<Response>(r => { resolve = () => r({ ok: true, json: () => Promise.resolve({ token: 't', expiresAt: new Date(Date.now() + 3600000).toISOString(), user: { id: '1', email: 'new@example.com' } }) } as Response) })
    ))
    const user = userEvent.setup()
    renderRegister()
    await user.type(screen.getByLabelText(/^email/i), 'new@example.com')
    await user.type(screen.getByLabelText(/^password$/i), 'Password1!')
    await user.type(screen.getByLabelText(/confirm password/i), 'Password1!')
    await user.click(screen.getByRole('button', { name: /create account/i }))
    await waitFor(() => expect(screen.getByRole('button', { name: /creating account/i })).toBeDisabled())
    resolve!()
  })
})
