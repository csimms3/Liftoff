import { useEffect, useRef, useState } from 'react'
import type { ApiService, ExerciseSet, SessionExercise } from '../api'
import { unlockAudio } from '../useRestTimer'

type SetApi = Pick<ApiService, 'patchSet' | 'createSet' | 'deleteSet'>
type WeightUnit = 'lbs' | 'kg'

const KG_PER_LB = 0.453592

// Weights are stored in lbs; the table shows and edits them in the user's unit.
function toDisplay(lbs: number, unit: WeightUnit): string {
  if (unit === 'lbs') return String(Math.round(lbs * 100) / 100)
  return String(Math.round(lbs * KG_PER_LB * 10) / 10)
}

function fromDisplay(value: number, unit: WeightUnit): number {
  return unit === 'lbs' ? value : Math.round((value / KG_PER_LB) * 100) / 100
}

interface Row {
  set: ExerciseSet // last saved state
  weight: string // what's in the inputs
  reps: string
  done: boolean // shown immediately; saved in the background
}

function toRow(set: ExerciseSet, unit: WeightUnit): Row {
  return { set, weight: toDisplay(set.weight, unit), reps: String(set.reps), done: set.completed }
}

interface SetTableProps {
  sessionExercise: SessionExercise
  api: SetApi
  weightUnit: WeightUnit
  onError: (message: string) => void
  /** Applies a change to the parent's copy of this exercise's sets. Called when the
   *  server confirms it, even if the table has unmounted meanwhile (e.g. the user
   *  switched views mid-save), since the table is rebuilt from that copy. */
  onSetsUpdate: (sessionExerciseId: string, update: (sets: ExerciseSet[]) => ExerciseSet[]) => void
  /** Called when a set is ticked done (not when it's unticked), e.g. to start the rest timer. */
  onSetDone?: () => void
}

const sameSets = (a: ExerciseSet[], b: ExerciseSet[]) =>
  a.length === b.length &&
  a.every((s, i) => s.id === b[i].id && s.weight === b[i].weight && s.reps === b[i].reps && s.completed === b[i].completed)

/**
 * One row per set: number, what you did last time, editable weight and reps, and
 * a check to mark it done. Edits save when a field loses focus; − and + after the
 * last set remove or add a set for this session.
 */
export function SetTable({ sessionExercise, api, weightUnit, onError, onSetsUpdate, onSetDone }: SetTableProps) {
  const [rows, setRows] = useState<Row[]>(() => sessionExercise.sets.map((s) => toRow(s, weightUnit)))
  const [busy, setBusy] = useState(false)
  // Saves for the same set run one after another, so a weight edit followed by a
  // quick tap on ✓ can't land out of order.
  const queues = useRef(new Map<string, Promise<unknown>>())
  const enqueue = <T,>(id: string, run: () => Promise<T>): Promise<T> => {
    const next = (queues.current.get(id) ?? Promise.resolve()).catch(() => {}).then(run)
    queues.current.set(id, next)
    return next
  }

  // Resync when the parent's copy differs from ours (a reload from the server).
  // Our own changes come back through the parent too; those match and are skipped,
  // so rows being edited aren't reset.
  const rowsRef = useRef(rows)
  rowsRef.current = rows
  useEffect(() => {
    if (sameSets(sessionExercise.sets, rowsRef.current.map((r) => r.set))) return
    // Rebuild only rows whose saved copy changed; others keep what's being typed.
    setRows((rs) =>
      sessionExercise.sets.map((s) => {
        const row = rs.find((r) => r.set.id === s.id)
        return row && sameSets([row.set], [s]) ? row : toRow(s, weightUnit)
      }),
    )
  }, [sessionExercise.sets, weightUnit])

  const lastUnit = useRef(weightUnit)
  useEffect(() => {
    if (lastUnit.current === weightUnit) return
    lastUnit.current = weightUnit
    setRows((rs) => rs.map((r) => ({ ...r, weight: toDisplay(r.set.weight, weightUnit) })))
  }, [weightUnit])

  const previous = sessionExercise.previous ?? []
  const name = sessionExercise.exercise?.name ?? 'exercise'

  const update = (id: string, change: (r: Row) => Row) =>
    setRows((rs) => rs.map((r) => (r.set.id === id ? change(r) : r)))

  // A confirmed save: record it here and in the parent. A field (or the tick) only
  // follows the server's value if it still shows the previous saved value.
  const saved = (set: ExerciseSet) => {
    update(set.id, (r) => ({
      ...r,
      set,
      done: r.done === r.set.completed ? set.completed : r.done,
      weight: r.weight === toDisplay(r.set.weight, weightUnit) ? toDisplay(set.weight, weightUnit) : r.weight,
      reps: r.reps === String(r.set.reps) ? String(set.reps) : r.reps,
    }))
    onSetsUpdate(sessionExercise.id, (sets) => sets.map((s) => (s.id === set.id ? set : s)))
  }

  const revertFields = (id: string) =>
    update(id, (r) => ({ ...r, weight: toDisplay(r.set.weight, weightUnit), reps: String(r.set.reps) }))

  const edit = (id: string, field: 'weight' | 'reps', value: string) => update(id, (r) => ({ ...r, [field]: value }))

  // The fields the user actually changed, parsed. `invalid` if either can't be
  // saved. Weight is compared as displayed: re-sending an untouched kg value would
  // round-trip it (100 lb -> 45.4 kg -> 100.09 lb).
  const changes = (row: Row) => {
    const out: { weight?: number; reps?: number; invalid?: boolean } = {}
    if (row.weight.trim() !== toDisplay(row.set.weight, weightUnit)) {
      const w = Number(row.weight)
      if (row.weight.trim() === '' || !Number.isFinite(w) || w < 0) out.invalid = true
      else out.weight = fromDisplay(w, weightUnit)
    }
    if (row.reps.trim() !== String(row.set.reps)) {
      const n = Number(row.reps)
      if (row.reps.trim() === '' || !Number.isInteger(n) || n < 0) out.invalid = true
      else out.reps = n
    }
    return out
  }

  const save = async (row: Row) => {
    const { weight, reps, invalid } = changes(row)
    if (invalid) {
      revertFields(row.set.id) // invalid entry: put the saved values back
      return
    }
    if (weight === undefined && reps === undefined) return
    try {
      saved(await enqueue(row.set.id, () => api.patchSet(row.set.id, { weight, reps })))
    } catch {
      revertFields(row.set.id)
      onError('Failed to save set')
    }
  }

  const toggle = async (row: Row) => {
    const done = !row.done
    const { weight, reps, invalid } = changes(row)
    update(row.set.id, (r) => ({ ...r, done }))
    if (done) unlockAudio() // inside the tap, so the rest beep is allowed on iOS
    const patch = { completed: done, ...(!invalid && { weight, reps }) }
    try {
      saved(await enqueue(row.set.id, () => api.patchSet(row.set.id, patch)))
      if (done) onSetDone?.() // only a set that actually saved starts the rest
    } catch {
      update(row.set.id, (r) => ({ ...r, done: !done }))
      onError('Failed to update set')
    }
  }

  const addSet = async () => {
    // Copy the last row as shown, including an edit whose save is still in flight.
    const last = rows[rows.length - 1]
    const edited = last ? changes(last) : {}
    const reps = !edited.invalid && edited.reps !== undefined ? edited.reps : (last?.set.reps ?? 0)
    const weight = !edited.invalid && edited.weight !== undefined ? edited.weight : (last?.set.weight ?? 0)
    setBusy(true)
    try {
      const created = await api.createSet(sessionExercise.id, reps, weight)
      const set: ExerciseSet = { ...created, reps, weight, completed: false }
      setRows((rs) => [...rs, toRow(set, weightUnit)])
      onSetsUpdate(sessionExercise.id, (sets) => [...sets, set])
    } catch {
      onError('Failed to add set')
    } finally {
      setBusy(false)
    }
  }

  const removeSet = async () => {
    const last = rows[rows.length - 1]
    if (!last || rows.length <= 1) return
    if (last.done && !window.confirm(`Set ${rows.length} of ${name} is already logged. Remove it?`)) return
    setBusy(true)
    try {
      // After any save still queued for it, so the two can't race.
      await enqueue(last.set.id, () => api.deleteSet(last.set.id))
      setRows((rs) => rs.filter((r) => r.set.id !== last.set.id))
      onSetsUpdate(sessionExercise.id, (sets) => sets.filter((s) => s.id !== last.set.id))
    } catch {
      onError('Failed to remove set')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="set-table" role="table" aria-label={`${name} sets`}>
      <div className="set-row set-row-header" role="row">
        <span role="columnheader">Set</span>
        <span role="columnheader">Previous</span>
        <span role="columnheader">{weightUnit}</span>
        <span role="columnheader">Reps</span>
        <span role="columnheader" aria-label="Done">✓</span>
      </div>
      {rows.map((row, i) => {
        const prev = previous[i]
        const n = i + 1
        return (
          <div key={row.set.id} className={`set-row${row.done ? ' done' : ''}`} role="row">
            <span className="set-num" role="cell">{n}</span>
            <span className="set-prev" role="cell">
              {prev ? `${toDisplay(prev.weight, weightUnit)} × ${prev.reps}` : '—'}
            </span>
            <span role="cell">
              <input
                type="number"
                inputMode="decimal"
                min="0"
                step="any"
                aria-label={`Set ${n} weight`}
                value={row.weight}
                onChange={(e) => edit(row.set.id, 'weight', e.target.value)}
                onBlur={() => save(row)}
                onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
              />
            </span>
            <span role="cell">
              <input
                type="number"
                inputMode="numeric"
                min="0"
                step="1"
                aria-label={`Set ${n} reps`}
                value={row.reps}
                onChange={(e) => edit(row.set.id, 'reps', e.target.value)}
                onBlur={() => save(row)}
                onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
              />
            </span>
            <span role="cell">
              <button
                type="button"
                className="set-check"
                aria-label={`Set ${n} done`}
                aria-pressed={row.done}
                onClick={() => toggle(row)}
              >
                ✓
              </button>
            </span>
          </div>
        )
      })}
      <div className="set-controls">
        <button
          type="button"
          className="set-control"
          aria-label={`Remove last ${name} set`}
          onClick={removeSet}
          disabled={busy || rows.length <= 1}
        >
          −
        </button>
        <button type="button" className="set-control" aria-label={`Add ${name} set`} onClick={addSet} disabled={busy}>
          +
        </button>
      </div>
    </div>
  )
}
