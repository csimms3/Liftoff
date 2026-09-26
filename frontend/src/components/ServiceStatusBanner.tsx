import { useEffect, useState } from 'react'

const POLL_MS = 15_000

// Shows why sign-in can't work right now (database or server down). Keeps checking
// /api/status, so it appears if an outage starts while the page is open and clears
// once it ends.
export function ServiceStatusBanner() {
  const [problem, setProblem] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | undefined

    const check = async () => {
      let message: string | null = null
      try {
        const res = await fetch('/api/status')
        if (!res.ok) {
          const body = await res.json().catch(() => ({}))
          message = body.error || 'Liftoff is having trouble right now. Please try again in a minute.'
        }
      } catch {
        message = "Can't reach the Liftoff server. Check your connection and try again."
      }
      if (cancelled) return
      setProblem(message)
      timer = setTimeout(check, POLL_MS)
    }

    check()
    return () => {
      cancelled = true
      if (timer) clearTimeout(timer)
    }
  }, [])

  if (!problem) return null
  return (
    <div className="auth-error auth-status-banner" role="alert">
      {problem}
    </div>
  )
}
