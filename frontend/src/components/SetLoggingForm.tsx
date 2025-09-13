import { useState } from 'react'
import { ExerciseSet } from '../api'

interface SetLoggingFormProps {
  set: ExerciseSet
  setIndex: number
  onLogSet: (setId: string, reps: number, weight: number, notes?: string) => Promise<void>
  loading?: boolean
}

export function SetLoggingForm({ set, setIndex, onLogSet, loading = false }: SetLoggingFormProps) {
  const [reps, setReps] = useState(set.reps.toString())
  const [weight, setWeight] = useState(set.weight.toString())
  const [notes, setNotes] = useState(set.notes || '')
  const [isLogging, setIsLogging] = useState(false)

  const handleLogSet = async () => {
    const repsNum = parseInt(reps)
    const weightNum = parseFloat(weight)

    // Validation
    if (isNaN(repsNum) || repsNum < 1) {
      alert('Please enter at least 1 rep')
      return
    }
    if (isNaN(weightNum) || weightNum <= 0) {
      alert('Please enter a weight greater than 0')
      return
    }

    try {
      setIsLogging(true)
      await onLogSet(set.id, repsNum, weightNum, notes.trim() || undefined)
    } catch (error) {
      console.error('Failed to log set:', error)
      alert('Failed to log set. Please try again.')
    } finally {
      setIsLogging(false)
    }
  }

  if (set.completed) {
    return (
      <div className="set-card completed">
        <span className="set-number">Set {setIndex + 1}</span>
        <span className="set-details">
          {set.reps} reps @ {set.weight} lbs
        </span>
        {set.notes && (
          <span className="set-notes">Notes: {set.notes}</span>
        )}
        <span className="completed-check">✓</span>
      </div>
    )
  }

  return (
    <div className="set-card logging-form">
      <span className="set-number">Set {setIndex + 1}</span>
      <div className="set-inputs">
        <div className="input-group">
          <label>Reps</label>
          <input
            type="number"
            min="1"
            value={reps}
            onChange={(e) => setReps(e.target.value)}
            disabled={loading || isLogging}
            placeholder="0"
          />
        </div>
        <div className="input-group">
          <label>Weight (lbs)</label>
          <input
            type="number"
            min="0.01"
            step="0.01"
            value={weight}
            onChange={(e) => setWeight(e.target.value)}
            disabled={loading || isLogging}
            placeholder="0.00"
          />
        </div>
      </div>
      <div className="set-actions">
        <button
          className="btn-primary"
          onClick={handleLogSet}
          disabled={loading || isLogging || !reps || !weight}
        >
          {isLogging ? 'Logging...' : 'Log Set'}
        </button>
      </div>
    </div>
  )
}
