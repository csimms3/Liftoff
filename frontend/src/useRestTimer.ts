import { useCallback, useEffect, useRef, useState } from 'react'

const KEY = 'liftoff-rest-timer'
const DONE_VISIBLE_MS = 8000

interface Stored {
  sessionId: string
  endsAt: number // epoch ms: timestamp-based, so it survives a locked screen or reload
  total: number // seconds, for the progress ring
  label: string
}

function load(sessionId: string): Stored | null {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return null
    const t = JSON.parse(raw) as Stored
    return t.sessionId === sessionId && Date.now() < t.endsAt + DONE_VISIBLE_MS ? t : null
  } catch {
    return null
  }
}

function save(t: Stored | null) {
  try {
    if (t) localStorage.setItem(KEY, JSON.stringify(t))
    else localStorage.removeItem(KEY)
  } catch {
    /* storage unavailable: the timer still works for this page load */
  }
}

let audio: AudioContext | null = null

/**
 * Create or resume the shared AudioContext. Call it from a tap: iOS only lets
 * audio start from a user gesture, so the beep minutes later needs a context
 * that a tap already unlocked.
 */
export function unlockAudio() {
  try {
    const Ctx = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
    if (!Ctx) return
    audio ??= new Ctx()
    if (audio.state === 'suspended') void audio.resume()
  } catch {
    /* audio unavailable */
  }
}

/** Beep and vibrate when the rest is over (both are best-effort: browsers may block them). */
export function alertRestOver() {
  try {
    navigator.vibrate?.([200, 100, 200])
  } catch {
    /* not supported */
  }
  try {
    unlockAudio()
    const ctx = audio
    if (!ctx) return
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()
    osc.frequency.value = 880
    gain.gain.setValueAtTime(0.15, ctx.currentTime)
    gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.5)
    osc.connect(gain).connect(ctx.destination)
    osc.start()
    osc.stop(ctx.currentTime + 0.5)
  } catch {
    /* audio blocked */
  }
}

export interface RestTimer {
  /** Seconds left, rounded up; null when no timer is showing. */
  remaining: number | null
  total: number
  label: string
  /** True from the moment it reaches zero until it hides itself. */
  over: boolean
  start: (seconds: number, label: string) => void
  adjust: (deltaSeconds: number) => void
  dismiss: () => void
}

/** A rest countdown for the given session, restored after a reload. */
export function useRestTimer(sessionId: string | undefined, onOver: () => void = alertRestOver): RestTimer {
  const [timer, setTimer] = useState<Stored | null>(() => (sessionId ? load(sessionId) : null))
  const [now, setNow] = useState(() => Date.now())
  const alerted = useRef<number | null>(null)

  // A different (or ended) session has no timer.
  useEffect(() => {
    setTimer(sessionId ? load(sessionId) : null)
  }, [sessionId])

  useEffect(() => {
    if (!timer) return
    const id = setInterval(() => setNow(Date.now()), 250)
    const onVisible = () => setNow(Date.now())
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      clearInterval(id)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [timer])

  const over = !!timer && now >= timer.endsAt

  useEffect(() => {
    if (!timer || !over) return
    if (alerted.current !== timer.endsAt) {
      alerted.current = timer.endsAt
      onOver()
    }
    if (now >= timer.endsAt + DONE_VISIBLE_MS) {
      save(null)
      setTimer(null)
    }
  }, [timer, over, now, onOver])

  const start = useCallback(
    (seconds: number, label: string) => {
      if (!sessionId || seconds <= 0) return
      const t = { sessionId, endsAt: Date.now() + seconds * 1000, total: seconds, label }
      save(t)
      setNow(Date.now())
      setTimer(t)
    },
    [sessionId],
  )

  const adjust = useCallback((delta: number) => {
    setTimer((t) => {
      if (!t) return t
      const next = { ...t, endsAt: Math.max(Date.now() + 1000, t.endsAt + delta * 1000), total: Math.max(1, t.total + delta) }
      save(next)
      return next
    })
  }, [])

  const dismiss = useCallback(() => {
    save(null)
    setTimer(null)
  }, [])

  return {
    remaining: timer ? Math.max(0, Math.ceil((timer.endsAt - now) / 1000)) : null,
    total: timer?.total ?? 0,
    label: timer?.label ?? '',
    over,
    start,
    adjust,
    dismiss,
  }
}
