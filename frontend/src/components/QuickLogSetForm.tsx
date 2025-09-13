import { useState } from 'react'

interface QuickLogSetFormProps {
  exerciseName: string
  plannedReps: number
  plannedWeight: number
  onLogSet: (reps: number, weight: number, notes?: string) => Promise<void>
  loading?: boolean
}

export function QuickLogSetForm({ exerciseName, plannedReps, plannedWeight, onLogSet, loading = false }: QuickLogSetFormProps) {
  const [reps, setReps] = useState(plannedReps.toString())
  const [weight, setWeight] = useState(plannedWeight.toString())
  const [notes, setNotes] = useState('')
  const [isLogging, setIsLogging] = useState(false)
  const [showForm, setShowForm] = useState(false)

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
      await onLogSet(repsNum, weightNum, notes.trim() || undefined)
      // Reset form after successful logging
      setReps(plannedReps.toString())
      setWeight(plannedWeight.toString())
      setNotes('')
      setShowForm(false)
    } catch (error) {
      console.error('Failed to log set:', error)
      alert('Failed to log set. Please try again.')
    } finally {
      setIsLogging(false)
    }
  }

  if (showForm) {
    return (
      <div className="quick-log-form">
        <h4>Log Set: {exerciseName}</h4>
        <div className="form-inputs">
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
          <div className="input-group">
            <label>Notes (optional)</label>
            <input
              type="text"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              disabled={loading || isLogging}
              placeholder="Add notes..."
            />
          </div>
        </div>
        <div className="form-actions">
          <button
            className="btn-secondary"
            onClick={() => setShowForm(false)}
            disabled={loading || isLogging}
          >
            Cancel
          </button>
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

  return (
    <div className="quick-log-trigger">
      <button
        className="btn-primary"
        onClick={() => setShowForm(true)}
        disabled={loading}
      >
        Quick Log Set
      </button>
    </div>
  )
}
