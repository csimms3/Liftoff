import { useEffect } from 'react'
import { formatRest } from '../formatRest'

const PRESETS = [0, 30, 60, 90, 120, 180, 300]

interface RestSettingDialogProps {
  name: string
  seconds: number
  onPick: (seconds: number) => void
  onClose: () => void
}

/** Choose an exercise's rest timer; it's remembered for that exercise everywhere. */
export function RestSettingDialog({ name, seconds, onPick, onClose }: RestSettingDialogProps) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])
  return (
    <div className="picker-backdrop" onClick={onClose}>
      <div className="picker-sheet" role="dialog" aria-label={`Rest timer for ${name}`} onClick={(e) => e.stopPropagation()}>
        <div className="picker-head">
          <h3>Rest after {name}</h3>
          <button type="button" className="picker-close" aria-label="Close" onClick={onClose}>×</button>
        </div>
        <div className="rest-options">
          {(PRESETS.includes(seconds) ? PRESETS : [...PRESETS, seconds].sort((a, b) => a - b)).map((value) => (
            <button key={value} type="button" className={`rest-option${value === seconds ? ' selected' : ''}`} aria-pressed={value === seconds} onClick={() => onPick(value)}>
              {formatRest(value)}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
