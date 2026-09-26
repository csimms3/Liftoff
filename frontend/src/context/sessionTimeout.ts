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
