import { useState } from 'react'
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
}

function App() {
  const [workouts, setWorkouts] = useState<Workout[]>([])
  const [currentWorkout, setCurrentWorkout] = useState<Workout | null>(null)
  const [newWorkoutName, setNewWorkoutName] = useState('')
  const [newExercise, setNewExercise] = useState({
    name: '',
    sets: 3,
    reps: 10,
    weight: 0
  })

  const createWorkout = () => {
    if (!newWorkoutName.trim()) return
    
    const workout: Workout = {
      id: Date.now().toString(),
      name: newWorkoutName,
      exercises: []
    }
    
    setWorkouts([...workouts, workout])
    setNewWorkoutName('')
  }

  const addExercise = () => {
    if (!currentWorkout || !newExercise.name.trim()) return
    
    const exercise: Exercise = {
      id: Date.now().toString(),
      ...newExercise
    }
    
    const updatedWorkout = {
      ...currentWorkout,
      exercises: [...currentWorkout.exercises, exercise]
    }
    
    setCurrentWorkout(updatedWorkout)
    setWorkouts(workouts.map(w => w.id === currentWorkout.id ? updatedWorkout : w))
    setNewExercise({ name: '', sets: 3, reps: 10, weight: 0 })
  }

  const startWorkout = (workout: Workout) => {
    setCurrentWorkout(workout)
  }

  return (
    <div className="app">
      <header className="app-header">
        <h1>🏋️ Liftoff</h1>
        <p>Track your workouts and build strength</p>
      </header>

      <main className="app-main">
        <div className="left-panel">
          <div className="workout-section">
            <h2>Create New Workout</h2>
            <div className="create-workout">
              <input
                type="text"
                placeholder="Workout name (e.g., Push Day)"
                value={newWorkoutName}
                onChange={(e) => setNewWorkoutName(e.target.value)}
                onKeyPress={(e) => e.key === 'Enter' && createWorkout()}
              />
              <button onClick={createWorkout}>Create Workout</button>
            </div>
          </div>

          <div className="workouts-section">
            <h2>Your Workouts</h2>
            {workouts.length === 0 ? (
              <p className="empty-state">No workouts yet. Create your first workout above!</p>
            ) : (
              <div className="workouts-grid">
                {workouts.map(workout => (
                  <div key={workout.id} className="workout-card">
                    <div>
                      <h3>{workout.name}</h3>
                      <p>{workout.exercises.length} exercises</p>
                    </div>
                    <button onClick={() => startWorkout(workout)}>
                      {currentWorkout?.id === workout.id ? 'Continue' : 'Start'}
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <div className="right-panel">
          {currentWorkout ? (
            <div className="current-workout">
              <h2>Current Workout: {currentWorkout.name}</h2>
              
              <div className="add-exercise">
                <h3>Add Exercise</h3>
                <div className="exercise-form">
                  <input
                    type="text"
                    placeholder="Exercise name"
                    value={newExercise.name}
                    onChange={(e) => setNewExercise({...newExercise, name: e.target.value})}
                  />
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
                    onChange={(e) => setNewExercise({...newExercise, weight: parseInt(e.target.value) || 0})}
                  />
                  <button onClick={addExercise}>Add Exercise</button>
                </div>
              </div>

              <div className="exercises-list">
                <h3>Exercises</h3>
                {currentWorkout.exercises.length === 0 ? (
                  <p className="empty-state">No exercises added yet. Add your first exercise above!</p>
                ) : (
                  <div className="exercises-grid">
                    {currentWorkout.exercises.map(exercise => (
                      <div key={exercise.id} className="exercise-card">
                        <div className="exercise-info">
                          <h4>{exercise.name}</h4>
                          <p>{exercise.sets} sets × {exercise.reps} reps</p>
                        </div>
                        <div className="exercise-stats">
                          {exercise.weight > 0 && <p>{exercise.weight} lbs</p>}
                        </div>
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
      </main>
    </div>
  )
}

export default App
