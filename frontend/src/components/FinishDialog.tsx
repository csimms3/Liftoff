import { useEffect, useRef, useState } from 'react'
import type { ApiService, SessionSummary } from '../api'

type FinishApi = Pick<ApiService, 'getSessionSummary'>

interface FinishDialogProps {
  api: FinishApi
  sessionId: string
  weightUnit: 'lbs' | 'kg'
  onFinish: (updateWorkout: boolean) => void
  onDiscard: () => void
  onClose: () => void
  busy?: boolean
  /** Why the last Finish/Discard failed; shown inside the dialog. */
  error?: string | null
}

const KG_PER_LB = 0.453592

function duration(seconds: number): string {
  const m = Math.max(0, Math.round(seconds / 60))
  return m < 60 ? `${m} min` : `${Math.floor(m / 60)} h ${m % 60} min`
}

/** The Finish step: what you did, what changed vs the workout, and what to keep. */
export function FinishDialog({ api, sessionId, weightUnit, onFinish, onDiscard, onClose, busy = false, error = null }: FinishDialogProps) {
  const [summary, setSummary] = useState<SessionSummary | null>(null)
  const [failed, setFailed] = useState(false)
  const [update, setUpdate] = useState(true)

  useEffect(() => {
    let cancelled = false
    api.getSessionSummary(sessionId).then((s) => !cancelled && setSummary(s)).catch(() => !cancelled && setFailed(true))
    return () => {
      cancelled = true
    }
  }, [api, sessionId])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  // Focus the dialog when it opens and hand focus back to what opened it.
  const dialog = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null
    dialog.current?.focus()
    return () => opener?.focus?.()
  }, [])

  const volume = summary ? (weightUnit === 'lbs' ? summary.volume : summary.volume * KG_PER_LB) : 0
  const nothingLogged = summary !== null && summary.sets_done === 0
  const c = summary?.changes
  const offerUpdate = !!summary && summary.can_update_workout && !!c?.has_changes && !nothingLogged

  return (
    <div className="picker-backdrop" onClick={onClose}>
      <div ref={dialog} tabIndex={-1} className="picker-sheet finish-dialog" role="dialog" aria-label="Finish workout" onClick={(e) => e.stopPropagation()}>
        <div className="picker-head">
          <h3>{nothingLogged ? 'Nothing logged' : 'Finish workout'}</h3>
          <button type="button" className="picker-close" aria-label="Close" onClick={onClose}>×</button>
        </div>

        {error && <p className="auth-error" role="alert">{error}</p>}

        {failed && (
          <>
            <p className="empty-state">Couldn't load the summary.</p>
            <div className="finish-actions">
              <button type="button" className="btn-primary" disabled={busy} onClick={() => onFinish(false)}>Finish anyway</button>
              <button type="button" className="btn-secondary" onClick={onClose}>Keep going</button>
            </div>
          </>
        )}

        {!summary && !failed && <p className="empty-state">Loading…</p>}

        {summary && (
          <>
            {nothingLogged ? (
              <p>You haven't ticked any sets in <strong>{summary.workout_name || 'this workout'}</strong>. Discard it, or keep it in your history.</p>
            ) : (
              <dl className="finish-stats">
                <div><dt>Time</dt><dd>{duration(summary.duration_seconds)}</dd></div>
                <div><dt>Sets</dt><dd>{summary.sets_done} / {summary.sets_total}</dd></div>
                <div><dt>Volume</dt><dd>{Math.round(volume).toLocaleString()} {weightUnit}</dd></div>
              </dl>
            )}

            {offerUpdate && c && (
              <label className="finish-update">
                <input type="checkbox" checked={update} onChange={(e) => setUpdate(e.target.checked)} />
                <span>
                  Update <strong>{summary.workout_name}</strong> with today's changes
                  <ul>
                    {c.added.map((n) => <li key={`a-${n}`}>Add {n}</li>)}
                    {c.removed.map((n) => <li key={`r-${n}`}>Remove {n}</li>)}
                    {c.set_counts.map((s) => <li key={`s-${s.name}`}>{s.name}: {s.from} → {s.to} sets</li>)}
                    {c.reordered && <li>New exercise order</li>}
                  </ul>
                </span>
              </label>
            )}

            <div className="finish-actions">
              {nothingLogged ? (
                <>
                  <button type="button" className="btn-danger" disabled={busy} onClick={onDiscard}>Discard workout</button>
                  <button type="button" className="btn-secondary" disabled={busy} onClick={() => onFinish(false)}>Save anyway</button>
                </>
              ) : (
                <button type="button" className="btn-primary" disabled={busy} onClick={() => onFinish(offerUpdate && update)}>Finish workout</button>
              )}
              <button type="button" className="btn-secondary" onClick={onClose}>Keep going</button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
