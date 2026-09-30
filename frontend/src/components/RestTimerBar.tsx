import type { RestTimer } from '../useRestTimer'

const fmt = (s: number) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`

/** Sticky rest countdown: −15 s, +15 s, Skip. */
export function RestTimerBar({ timer }: { timer: RestTimer }) {
  if (timer.remaining === null) return null
  const pct = timer.total > 0 ? Math.min(100, Math.max(0, (1 - timer.remaining / timer.total) * 100)) : 100
  return (
    <div className={`rest-bar${timer.over ? ' over' : ''}`} role="timer" aria-live="off" aria-label="Rest timer">
      <div className="rest-progress" style={{ width: `${pct}%` }} />
      <div className="rest-content">
        <div className="rest-text">
          <span className="rest-time">{timer.over ? 'Rest over' : fmt(timer.remaining)}</span>
          <small>{timer.label}</small>
        </div>
        <div className="rest-actions">
          {!timer.over && (
            <>
              <button type="button" onClick={() => timer.adjust(-15)} aria-label="15 seconds less">−15</button>
              <button type="button" onClick={() => timer.adjust(15)} aria-label="15 seconds more">+15</button>
            </>
          )}
          <button type="button" onClick={timer.dismiss}>{timer.over ? 'Dismiss' : 'Skip'}</button>
        </div>
      </div>
    </div>
  )
}
