import { useEffect, useRef, useState } from 'react'
import type { ExerciseTemplate, Routine, Workout } from '../api'

/** Where a library exercise goes: the running session, or a workout's plan. */
export type QuickAddDestination = { kind: 'session' } | { kind: 'workout'; workoutId: string }

interface QuickAddDialogProps {
  template: ExerciseTemplate
  weightUnit: 'lbs' | 'kg'
  /** Weight shown in the user's unit (templates are stored in lbs). */
  displayWeight: number
  workouts: Workout[]
  routines: Routine[]
  /** Name of the active session's workout, if a session is running. */
  sessionName: string | null
  /** The workout preselected when there is no session. */
  defaultWorkoutId: string | null
  /** Rejects if the add failed, which keeps the dialog open. */
  onAdd: (destination: QuickAddDestination) => Promise<void>
  onClose: () => void
}

const SESSION = 'session'
const show = (n: number) => String(Math.round(n * 10) / 10)

/** Quick Add from the library: pick where the exercise goes, then add it. */
export function QuickAddDialog({ template, weightUnit, displayWeight, workouts, routines, sessionName, defaultWorkoutId, onAdd, onClose }: QuickAddDialogProps) {
  const hasSession = sessionName !== null
  const initial = hasSession ? SESSION : (workouts.find(w => w.id === defaultWorkoutId) ?? workouts[0])?.id ?? ''
  const [choice, setChoice] = useState(initial)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const dialog = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null
    dialog.current?.focus()
    return () => opener?.focus?.()
  }, [])

  // Workouts arrive ordered by routine, then position; group them in that order.
  const groups: { id: string; name: string; workouts: Workout[] }[] = []
  for (const w of workouts) {
    let g = groups.find(x => x.id === w.routine_id)
    if (!g) {
      g = { id: w.routine_id, name: routines.find(r => r.id === w.routine_id)?.name ?? 'Other workouts', workouts: [] }
      groups.push(g)
    }
    g.workouts.push(w)
  }

  const canAdd = hasSession || workouts.length > 0

  const add = async () => {
    setBusy(true)
    setError(null)
    try {
      await onAdd(choice === SESSION ? { kind: 'session' } : { kind: 'workout', workoutId: choice })
    } catch {
      setError('Failed to add exercise')
      setBusy(false)
    }
  }

  return (
    <div className="picker-backdrop" onClick={onClose}>
      <div ref={dialog} tabIndex={-1} className="picker-sheet quick-add-dialog" role="dialog" aria-label="Add to workout" onClick={(e) => e.stopPropagation()}>
        <div className="picker-head">
          <h3>{template.name}</h3>
          <button type="button" className="picker-close" aria-label="Close" onClick={onClose}>×</button>
        </div>
        <p className="quick-add-defaults">{template.default_sets} × {template.default_reps} @ {show(displayWeight)} {weightUnit}</p>

        {error && <p className="auth-error" role="alert">{error}</p>}

        {canAdd ? (
          <label className="quick-add-field">
            <span>Add to</span>
            <select value={choice} onChange={(e) => setChoice(e.target.value)} disabled={busy}>
              {hasSession && <option value={SESSION}>Current session: {sessionName}</option>}
              {groups.map(g => (
                <optgroup key={g.id} label={g.name}>
                  {g.workouts.map(w => <option key={w.id} value={w.id}>{w.name}</option>)}
                </optgroup>
              ))}
            </select>
          </label>
        ) : (
          <p className="empty-state">You don't have any workouts yet. Create one in the Workouts tab, then come back to add this exercise.</p>
        )}

        <div className="finish-actions">
          {canAdd && <button type="button" className="btn-primary" disabled={busy} onClick={add}>Add</button>}
          <button type="button" className="btn-secondary" onClick={onClose}>{canAdd ? 'Cancel' : 'Close'}</button>
        </div>
      </div>
    </div>
  )
}
