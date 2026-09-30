import { useEffect, useState, type ReactNode } from 'react'
import type { SessionExercise } from '../api'
import { formatElapsed, sessionStats } from '../sessionStats'

const KG_PER_LB = 0.453592

interface SessionHeaderProps {
  workoutName: string
  startedAt: string
  exercises: SessionExercise[]
  weightUnit: 'lbs' | 'kg'
  /** The primary action, e.g. Finish. */
  action: ReactNode
}

/** Sticky session header: workout, live elapsed time, sets and volume, and the action. */
export function SessionHeader({ workoutName, startedAt, exercises, weightUnit, action }: SessionHeaderProps) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000)
    const onVisible = () => setNow(Date.now())
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      clearInterval(id)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [])

  const started = new Date(startedAt).getTime()
  const { done, total, volume } = sessionStats(exercises)
  const shown = Math.round(weightUnit === 'lbs' ? volume : volume * KG_PER_LB)

  return (
    <div className="session-header">
      <div className="session-header-main">
        <h2>{workoutName}</h2>
        <dl className="session-stats">
          <div><dt>Time</dt><dd aria-label="Elapsed time">{Number.isNaN(started) ? '–' : formatElapsed(now - started)}</dd></div>
          <div><dt>Sets</dt><dd>{done}/{total}</dd></div>
          <div><dt>Volume</dt><dd>{shown.toLocaleString()} {weightUnit}</dd></div>
        </dl>
      </div>
      {action}
    </div>
  )
}
