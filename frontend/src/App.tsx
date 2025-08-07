import { useState, useEffect } from 'react'
import './App.css'

interface Exercise {
  id: string
  name: string
  sets: number
  reps: number
  weight: number
}

interface Workout {
  id: string
  name: string
  exercises: Exercise[]
  createdAt: string
}

interface WorkoutSession {
  id: string
  workoutId: string
  workout: Workout
  startedAt: string
  endedAt?: string
  isActive: boolean
  exercises: SessionExercise[]
}

interface SessionExercise {
  id: string
  exerciseId: string
  exercise: Exercise
  sets: ExerciseSet[]
}

interface ExerciseSet {
  id: string
  reps: number
  weight: number
  completed: boolean
  notes?: string
}

interface ProgressData {
  exerciseName: string
  date: string
  maxWeight: number
  totalVolume: number
}

function App() {
  const [workouts, setWorkouts] = useState<Workout[]>([])
  const [currentWorkout, setCurrentWorkout] = useState<Workout | null>(null)
  const [activeSession, setActiveSession] = useState<WorkoutSession | null>(null)
  const [progressData, setProgressData] = useState<ProgressData[]>([])
  const [view, setView] = useState<'workouts' | 'session' | 'progress'>('workouts')
  
  const [newWorkoutName, setNewWorkoutName] = useState('')
  const [newExercise, setNewExercise] = useState({
    name: '',
    sets: 3,
    reps: 10,
    weight: 0
  })

  // Load data from localStorage on mount
  useEffect(() => {
    const savedWorkouts = localStorage.getItem('liftoff-workouts')
    const savedSessions = localStorage.getItem('liftoff-sessions')
    const savedProgress = localStorage.getItem('liftoff-progress')
    
    if (savedWorkouts) {
      setWorkouts(JSON.parse(savedWorkouts))
    }
    if (savedSessions) {
      const sessions = JSON.parse(savedSessions)
      const active = sessions.find((s: WorkoutSession) => s.isActive)
      if (active) setActiveSession(active)
    }
    if (savedProgress) {
      setProgressData(JSON.parse(savedProgress))
    }
  }, [])

  // Save data to localStorage whenever it changes
  useEffect(() => {
    localStorage.setItem('liftoff-workouts', JSON.stringify(workouts))
  }, [workouts])

  useEffect(() => {
    if (activeSession) {
      const sessions = JSON.parse(localStorage.getItem('liftoff-sessions') || '[]')
      const updatedSessions = sessions.filter((s: WorkoutSession) => s.id !== activeSession.id)
      updatedSessions.push(activeSession)
      localStorage.setItem('liftoff-sessions', JSON.stringify(updatedSessions))
    }
  }, [activeSession])

  const createWorkout = () => {
    if (!newWorkoutName.trim()) return
    
    const workout: Workout = {
      id: Date.now().toString(),
      name: newWorkoutName.trim(),
      exercises: [],
      createdAt: new Date().toISOString()
    }
    
    setWorkouts([workout, ...workouts])
    setNewWorkoutName('')
  }

  const addExercise = () => {
    if (!newExercise.name.trim() || !currentWorkout) return
    
    const exercise: Exercise = {
      id: Date.now().toString(),
      name: newExercise.name.trim(),
      sets: newExercise.sets,
      reps: newExercise.reps,
      weight: newExercise.weight
    }
    
    const updatedWorkout = {
      ...currentWorkout,
      exercises: [...currentWorkout.exercises, exercise]
    }
    
    setWorkouts(workouts.map(w => w.id === currentWorkout.id ? updatedWorkout : w))
    setCurrentWorkout(updatedWorkout)
    
    setNewExercise({
      name: '',
      sets: 3,
      reps: 10,
      weight: 0
    })
  }

  const startWorkout = (workout: Workout) => {
    if (activeSession) {
      // End current session first
      const endedSession = { ...activeSession, isActive: false, endedAt: new Date().toISOString() }
      setActiveSession(null)
    }
    
    const session: WorkoutSession = {
      id: Date.now().toString(),
      workoutId: workout.id,
      workout: workout,
      startedAt: new Date().toISOString(),
      isActive: true,
      exercises: workout.exercises.map(exercise => ({
        id: Date.now().toString() + Math.random(),
        exerciseId: exercise.id,
        exercise: exercise,
        sets: Array.from({ length: exercise.sets }, (_, i) => ({
          id: Date.now().toString() + i,
          reps: exercise.reps,
          weight: exercise.weight,
          completed: false
        }))
      }))
    }
    
    setActiveSession(session)
    setView('session')
  }

  const completeSet = (sessionExerciseId: string, setIndex: number) => {
    if (!activeSession) return
    
    const updatedSession = {
      ...activeSession,
      exercises: activeSession.exercises.map(ex => {
        if (ex.id === sessionExerciseId) {
          const updatedSets = [...ex.sets]
          updatedSets[setIndex] = { ...updatedSets[setIndex], completed: true }
          return { ...ex, sets: updatedSets }
        }
        return ex
      })
    }
    
    setActiveSession(updatedSession)
  }

  const endSession = () => {
    if (!activeSession) return
    
    const endedSession = {
      ...activeSession,
      isActive: false,
      endedAt: new Date().toISOString()
    }
    
    // Calculate progress data
    const newProgress: ProgressData[] = []
    activeSession.exercises.forEach(ex => {
      const maxWeight = Math.max(...ex.sets.map(s => s.weight))
      const totalVolume = ex.sets.reduce((sum, s) => sum + (s.weight * s.reps), 0)
      
      newProgress.push({
        exerciseName: ex.exercise.name,
        date: new Date().toISOString().split('T')[0],
        maxWeight,
        totalVolume
      })
    })
    
    setProgressData([...progressData, ...newProgress])
    localStorage.setItem('liftoff-progress', JSON.stringify([...progressData, ...newProgress]))
    
    setActiveSession(null)
    setView('workouts')
  }

  const deleteWorkout = (workoutId: string) => {
    setWorkouts(workouts.filter(w => w.id !== workoutId))
    if (currentWorkout?.id === workoutId) {
      setCurrentWorkout(null)
    }
  }

  const deleteExercise = (exerciseId: string) => {
    if (!currentWorkout) return
    
    const updatedWorkout = {
      ...currentWorkout,
      exercises: currentWorkout.exercises.filter(e => e.id !== exerciseId)
    }
    
    setWorkouts(workouts.map(w => w.id === currentWorkout.id ? updatedWorkout : w))
    setCurrentWorkout(updatedWorkout)
  }

  return (
    <div className="app">
      <header className="app-header">
        <h1>🏋️ Liftoff</h1>
        <p>Track your workouts and build strength</p>
        <nav className="app-nav">
          <button 
            className={`nav-button ${view === 'workouts' ? 'active' : ''}`}
            onClick={() => setView('workouts')}
          >
            Workouts
          </button>
          <button 
            className={`nav-button ${view === 'session' ? 'active' : ''}`}
            onClick={() => setView('session')}
            disabled={!activeSession}
          >
            Active Session
          </button>
          <button 
            className={`nav-button ${view === 'progress' ? 'active' : ''}`}
            onClick={() => setView('progress')}
          >
            Progress
          </button>
        </nav>
      </header>

      <main className="app-main">
        {view === 'workouts' && (
          <div className="workouts-view">
            <div className="left-panel">
              <div className="workout-section">
                <h2>Create New Workout</h2>
                <div className="input-group">
                  <input
                    type="text"
                    placeholder="Workout name (e.g., Push Day)"
                    value={newWorkoutName}
                    onChange={(e) => setNewWorkoutName(e.target.value)}
                    onKeyPress={(e) => e.key === 'Enter' && createWorkout()}
                  />
                  <button onClick={createWorkout} className="btn-primary">
                    Create Workout
                  </button>
                </div>
              </div>

              <div className="workouts-section">
                <h2>Your Workouts</h2>
                {workouts.length === 0 ? (
                  <p className="empty-state">No workouts yet. Create your first workout above!</p>
                ) : (
                  <div className="workout-cards">
                    {workouts.map(workout => (
                      <div key={workout.id} className="workout-card">
                        <div className="workout-header">
                          <h3>{workout.name}</h3>
                          <button 
                            onClick={() => deleteWorkout(workout.id)}
                            className="btn-delete"
                            title="Delete workout"
                          >
                            ×
                          </button>
                        </div>
                        <p className="workout-stats">
                          {workout.exercises.length} exercises
                        </p>
                        <div className="workout-actions">
                          <button 
                            onClick={() => setCurrentWorkout(workout)}
                            className="btn-secondary"
                          >
                            Edit
                          </button>
                          <button 
                            onClick={() => startWorkout(workout)}
                            className="btn-primary"
                            disabled={activeSession?.isActive}
                          >
                            {activeSession?.workoutId === workout.id ? 'Continue' : 'Start'}
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>

            <div className="right-panel">
              {currentWorkout ? (
                <div className="current-workout">
                  <h2>Edit Workout: {currentWorkout.name}</h2>
                  <div className="add-exercise">
                    <h3>Add Exercise</h3>
                    <div className="exercise-form">
                      <input
                        type="text"
                        placeholder="Exercise name"
                        value={newExercise.name}
                        onChange={(e) => setNewExercise({...newExercise, name: e.target.value})}
                      />
                      <div className="exercise-inputs">
                        <input
                          type="number"
                          placeholder="Sets"
                          value={newExercise.sets}
                          onChange={(e) => setNewExercise({...newExercise, sets: parseInt(e.target.value) || 0})}
                        />
                        <input
                          type="number"
                          placeholder="Reps"
                          value={newExercise.reps}
                          onChange={(e) => setNewExercise({...newExercise, reps: parseInt(e.target.value) || 0})}
                        />
                        <input
                          type="number"
                          placeholder="Weight (lbs)"
                          value={newExercise.weight}
                          onChange={(e) => setNewExercise({...newExercise, weight: parseFloat(e.target.value) || 0})}
                        />
                      </div>
                      <button onClick={addExercise} className="btn-primary">
                        Add Exercise
                      </button>
                    </div>
                  </div>
                  <div className="exercises-list">
                    <h3>Exercises</h3>
                    {currentWorkout.exercises.length === 0 ? (
                      <p className="empty-state">No exercises added yet. Add your first exercise above!</p>
                    ) : (
                      <div className="exercise-cards">
                        {currentWorkout.exercises.map(exercise => (
                          <div key={exercise.id} className="exercise-card">
                            <div className="exercise-header">
                              <h4>{exercise.name}</h4>
                              <button 
                                onClick={() => deleteExercise(exercise.id)}
                                className="btn-delete-small"
                                title="Delete exercise"
                              >
                                ×
                              </button>
                            </div>
                            <p className="exercise-stats">
                              {exercise.sets} sets × {exercise.reps} reps
                              {exercise.weight > 0 && ` @ ${exercise.weight} lbs`}
                            </p>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              ) : (
                <div className="current-workout">
                  <h2>Select a Workout</h2>
                  <p className="empty-state">Choose a workout from the left panel to start tracking your exercises.</p>
                </div>
              )}
            </div>
          </div>
        )}

        {view === 'session' && activeSession && (
          <div className="session-view">
            <div className="session-header">
              <h2>Active Session: {activeSession.workout.name}</h2>
              <div className="session-info">
                <span>Started: {new Date(activeSession.startedAt).toLocaleTimeString()}</span>
                <button onClick={endSession} className="btn-danger">
                  End Session
                </button>
              </div>
            </div>
            
            <div className="session-exercises">
              {activeSession.exercises.map(sessionExercise => (
                <div key={sessionExercise.id} className="session-exercise">
                  <h3>{sessionExercise.exercise.name}</h3>
                  <div className="sets-grid">
                    {sessionExercise.sets.map((set, index) => (
                      <div 
                        key={set.id} 
                        className={`set-card ${set.completed ? 'completed' : ''}`}
                        onClick={() => completeSet(sessionExercise.id, index)}
                      >
                        <span className="set-number">Set {index + 1}</span>
                        <span className="set-details">
                          {set.reps} reps @ {set.weight} lbs
                        </span>
                        {set.completed && <span className="completed-check">✓</span>}
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {view === 'progress' && (
          <div className="progress-view">
            <h2>Progress Tracking</h2>
            {progressData.length === 0 ? (
              <p className="empty-state">No progress data yet. Complete some workouts to see your progress!</p>
            ) : (
              <div className="progress-charts">
                <div className="progress-summary">
                  <h3>Recent Activity</h3>
                  <div className="progress-cards">
                    {progressData.slice(-5).reverse().map((data, index) => (
                      <div key={index} className="progress-card">
                        <h4>{data.exerciseName}</h4>
                        <p className="progress-date">{new Date(data.date).toLocaleDateString()}</p>
                        <div className="progress-stats">
                          <span>Max Weight: {data.maxWeight} lbs</span>
                          <span>Volume: {data.totalVolume} lbs</span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </main>
    </div>
  )
}

export default App
