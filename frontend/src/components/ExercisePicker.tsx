import { useEffect, useMemo, useRef, useState } from 'react'
import type { ApiService, ExerciseTemplate, MovementSummary } from '../api'

type PickerApi = Pick<ApiService, 'getMovements' | 'getExerciseTemplates'>

export type PickTarget = { movement_id: string } | { name: string }

interface ExercisePickerProps {
  api: PickerApi
  title: string
  onPick: (target: PickTarget) => void
  onClose: () => void
}

const norm = (s: string) => s.trim().toLowerCase()

/**
 * Bottom sheet to choose an exercise: your own movements (most recent first),
 * then built-in library exercises you haven't used, then "Create <typed name>".
 */
export function ExercisePicker({ api, title, onPick, onClose }: ExercisePickerProps) {
  const [movements, setMovements] = useState<MovementSummary[]>([])
  const [library, setLibrary] = useState<ExerciseTemplate[]>([])
  const [query, setQuery] = useState('')
  const [failed, setFailed] = useState(false)
  const input = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null
    input.current?.focus()
    let cancelled = false
    Promise.all([api.getMovements(), api.getExerciseTemplates()])
      .then(([m, l]) => {
        if (cancelled) return
        setMovements(m)
        setLibrary(l)
      })
      .catch(() => !cancelled && setFailed(true))
    return () => {
      cancelled = true
      opener?.focus?.() // back to the button that opened the sheet
    }
  }, [api])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const q = norm(query)
  const mine = useMemo(() => movements.filter((m) => norm(m.name).includes(q)), [movements, q])
  const known = useMemo(() => new Set(movements.map((m) => norm(m.name))), [movements])
  const fromLibrary = useMemo(
    () => library.filter((t) => !known.has(norm(t.name)) && norm(t.name).includes(q)),
    [library, known, q],
  )
  const exact = q !== '' && (known.has(q) || library.some((t) => norm(t.name) === q))

  return (
    <div className="picker-backdrop" onClick={onClose}>
      <div className="picker-sheet" role="dialog" aria-label={title} onClick={(e) => e.stopPropagation()}>
        <div className="picker-head">
          <h3>{title}</h3>
          <button type="button" className="picker-close" aria-label="Close" onClick={onClose}>×</button>
        </div>
        <input
          ref={input}
          className="picker-search"
          type="search"
          placeholder="Search or type a new exercise"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key !== 'Enter' || q === '') return
            // Enter picks the exact match, else the only match, else creates the typed name.
            const options: { target: PickTarget; name: string }[] = [
              ...mine.map((m) => ({ target: { movement_id: m.id } as PickTarget, name: m.name })),
              ...fromLibrary.map((t) => ({ target: { name: t.name } as PickTarget, name: t.name })),
            ]
            const hit = options.find((o) => norm(o.name) === q) ?? (options.length === 1 ? options[0] : undefined)
            onPick(hit ? hit.target : { name: query.trim() })
          }}
        />
        <div className="picker-list">
          {failed && <p className="empty-state">Couldn't load exercises. You can still type a name.</p>}
          {q !== '' && !exact && (
            <button type="button" className="picker-item picker-create" onClick={() => onPick({ name: query.trim() })}>
              Create “{query.trim()}”
            </button>
          )}
          {mine.length > 0 && <p className="picker-section">Your exercises</p>}
          {mine.map((m) => (
            <button key={m.id} type="button" className="picker-item" onClick={() => onPick({ movement_id: m.id })}>
              <span>{m.name}</span>
              {m.category && <small>{m.category}</small>}
            </button>
          ))}
          {fromLibrary.length > 0 && <p className="picker-section">Library</p>}
          {fromLibrary.map((t) => (
            <button key={t.name} type="button" className="picker-item" onClick={() => onPick({ name: t.name })}>
              <span>{t.name}</span>
              <small>{t.category}</small>
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
