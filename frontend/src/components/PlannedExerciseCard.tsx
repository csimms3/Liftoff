import { useState } from 'react'
import type { Exercise } from '../api'

type Unit = 'lbs' | 'kg'

interface PlannedExerciseCardProps {
  exercise: Exercise
  weightUnit: Unit
  convertWeight: (weight: number, from: Unit, to: Unit) => number
  canMoveUp: boolean
  canMoveDown: boolean
  disabled?: boolean
  /** Rejects if the save failed, which keeps the editor open. Weight is in lbs. */
  onSave: (patch: { sets?: number; reps?: number; weight?: number }) => Promise<void>
  onMove: (delta: -1 | 1) => void
  onRemove: () => void
}

const show = (n: number) => String(Math.round(n * 10) / 10)

/** A workout exercise's plan (sets x reps @ weight) with inline editing, reordering and removal. */
export function PlannedExerciseCard({ exercise, weightUnit, convertWeight, canMoveUp, canMoveDown, disabled = false, onSave, onMove, onRemove }: PlannedExerciseCardProps) {
  const shownWeight = show(convertWeight(exercise.weight, 'lbs', weightUnit))
  const [editing, setEditing] = useState(false)
  const [sets, setSets] = useState('')
  const [reps, setReps] = useState('')
  const [weight, setWeight] = useState('')

  const edit = () => {
    setSets(String(exercise.sets))
    setReps(String(exercise.reps))
    setWeight(shownWeight)
    setEditing(true)
  }

  const save = async () => {
    const patch: { sets?: number; reps?: number; weight?: number } = {}
    const s = Number(sets)
    const r = Number(reps)
    const w = parseFloat(weight)
    if (s !== exercise.sets) patch.sets = s
    if (r !== exercise.reps) patch.reps = r
    // Weights are stored in lbs; only send one the user actually changed, so a rounded kg value doesn't drift.
    if (w >= 0 && show(w) !== shownWeight) patch.weight = convertWeight(w, weightUnit, 'lbs')
    if (Object.keys(patch).length > 0) {
      try {
        await onSave(patch)
      } catch {
        return
      }
    }
    setEditing(false)
  }

  const wholeAtLeastOne = (v: string) => v.trim() !== '' && Number.isInteger(Number(v)) && Number(v) >= 1
  const valid = wholeAtLeastOne(sets) && wholeAtLeastOne(reps) && parseFloat(weight) >= 0

  return (
    <div className="exercise-card">
      <div className="exercise-header">
        <h4>{exercise.name}</h4>
        <div className="exercise-card-actions">
          <button type="button" className="icon-button" aria-label={`Move ${exercise.name} up`} disabled={disabled || !canMoveUp} onClick={() => onMove(-1)}>↑</button>
          <button type="button" className="icon-button" aria-label={`Move ${exercise.name} down`} disabled={disabled || !canMoveDown} onClick={() => onMove(1)}>↓</button>
          <button type="button" className="btn-delete-small" aria-label={`Remove ${exercise.name}`} disabled={disabled} onClick={onRemove}>×</button>
        </div>
      </div>
      {editing ? (
        <form className="exercise-inputs" onSubmit={(e) => { e.preventDefault(); if (valid) void save() }}>
          <div className="input-group">
            <label htmlFor={`sets-${exercise.id}`}>Sets</label>
            <input id={`sets-${exercise.id}`} type="number" min={1} step={1} value={sets} onChange={(e) => setSets(e.target.value)} />
          </div>
          <div className="input-group">
            <label htmlFor={`reps-${exercise.id}`}>Reps</label>
            <input id={`reps-${exercise.id}`} type="number" min={1} step={1} value={reps} onChange={(e) => setReps(e.target.value)} />
          </div>
          <div className="input-group">
            <label htmlFor={`weight-${exercise.id}`}>Weight ({weightUnit})</label>
            <input id={`weight-${exercise.id}`} type="number" min={0} step="any" value={weight} onChange={(e) => setWeight(e.target.value)} />
          </div>
          <div className="exercise-edit-actions">
            <button type="submit" className="btn-primary" disabled={!valid}>Save</button>
            <button type="button" className="btn-secondary" onClick={() => setEditing(false)}>Cancel</button>
          </div>
        </form>
      ) : (
        <div className="exercise-stats">
          <span>{`${exercise.sets} × ${exercise.reps}${exercise.weight > 0 ? ` @ ${shownWeight} ${weightUnit}` : ''}`}</span>
          <button type="button" className="btn-secondary" aria-label={`Edit ${exercise.name}`} disabled={disabled} onClick={edit}>Edit</button>
        </div>
      )}
    </div>
  )
}
