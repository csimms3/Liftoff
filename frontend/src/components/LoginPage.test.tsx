import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AuthProvider } from '../context/AuthContext'
import { LoginPage } from './LoginPage'

describe('LoginPage', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  it('renders login form', () => {
    render(
      <AuthProvider>
        <LoginPage onSwitchToRegister={() => {}} />
      </AuthProvider>
    )
    expect(screen.getByRole('heading', { name: /liftoff/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
    expect(screen.getByText(/create account/i)).toBeInTheDocument()
  })

  it('shows forgot password link when handler provided', () => {
    render(
      <AuthProvider>
        <LoginPage onSwitchToRegister={() => {}} onSwitchToForgotPassword={() => {}} />
      </AuthProvider>
    )
    expect(screen.getByText(/forgot password/i)).toBeInTheDocument()
  })

  it('calls onSwitchToRegister when create account clicked', async () => {
    const user = userEvent.setup()
    const onSwitch = vi.fn()
    render(
      <AuthProvider>
        <LoginPage onSwitchToRegister={onSwitch} />
      </AuthProvider>
    )
    await user.click(screen.getByText(/create account/i))
    expect(onSwitch).toHaveBeenCalled()
  })

  it('calls onSwitchToForgotPassword when forgot password clicked', async () => {
    const user = userEvent.setup()
    const onForgot = vi.fn()
    render(
      <AuthProvider>
        <LoginPage onSwitchToRegister={() => {}} onSwitchToForgotPassword={onForgot} />
      </AuthProvider>
    )
    await user.click(screen.getByText(/forgot password/i))
    expect(onForgot).toHaveBeenCalled()
  })
})
