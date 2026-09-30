import { ServiceStatusBanner } from './ServiceStatusBanner'

interface AuthLayoutProps {
  children: React.ReactNode
}

export function AuthLayout({ children }: AuthLayoutProps) {
  return (
    <div className="auth-layout">
      <ServiceStatusBanner />
      {children}
    </div>
  )
}
