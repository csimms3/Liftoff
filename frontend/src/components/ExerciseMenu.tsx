import { useEffect, useRef, useState } from 'react'

interface ExerciseMenuProps {
  name: string
  canMoveUp: boolean
  canMoveDown: boolean
  onMoveUp: () => void
  onMoveDown: () => void
  onReplace: () => void
  onRest?: () => void
  restLabel?: string
  onRemove: () => void
}

/** The ⋯ menu on a session exercise: move up/down, replace, remove. */
export function ExerciseMenu({ name, canMoveUp, canMoveDown, onMoveUp, onMoveDown, onReplace, onRest, restLabel, onRemove }: ExerciseMenuProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const close = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent ? e.key === 'Escape' : !ref.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', close)
    return () => {
      document.removeEventListener('mousedown', close)
      document.removeEventListener('keydown', close)
    }
  }, [open])

  const act = (fn: () => void) => () => {
    setOpen(false)
    fn()
  }

  return (
    <div className="exercise-menu" ref={ref}>
      <button type="button" className="exercise-menu-button" aria-label={`${name} options`} aria-expanded={open} onClick={() => setOpen((o) => !o)}>
        ⋯
      </button>
      {open && (
        <div className="exercise-menu-list" role="menu">
          <button type="button" role="menuitem" disabled={!canMoveUp} onClick={act(onMoveUp)}>Move up</button>
          <button type="button" role="menuitem" disabled={!canMoveDown} onClick={act(onMoveDown)}>Move down</button>
          <button type="button" role="menuitem" onClick={act(onReplace)}>Replace…</button>
          {onRest && <button type="button" role="menuitem" onClick={act(onRest)}>{restLabel ?? 'Rest timer…'}</button>}
          <button type="button" role="menuitem" className="danger" onClick={act(onRemove)}>Remove</button>
        </div>
      )}
    </div>
  )
}
