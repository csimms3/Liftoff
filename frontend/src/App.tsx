import React, { useState, useEffect } from 'react';
import './App.css'
import { ApiService } from './api'
import type { Workout, Exercise, WorkoutSession, WorkoutTemplate } from './api'
import { WorkoutLibrary } from './components/WorkoutLibrary';

interface ProgressData {
  exerciseName: string
  date: string
  maxWeight: number
  totalVolume: number
}

export default function App() {
  const apiService = new ApiService();
  
  const [view, setView] = useState<'workouts' | 'session' | 'progress' | 'library'>('workouts');
  const [workouts, setWorkouts] = useState<Workout[]>([]);
  const [currentWorkout, setCurrentWorkout] = useState<Workout | null>(null);
  const [activeSession, setActiveSession] = useState<WorkoutSession | null>(null);
  const [progressData, setProgressData] = useState<ProgressData[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const [newWorkoutName, setNewWorkoutName] = useState('')
  const [newExercise, setNewExercise] = useState({
    name: '',
    sets: 3,
    reps: 10,
    weight: 0
  })
  const [templates, setTemplates] = useState<WorkoutTemplate[]>([]);
  const [selectedTemplate, setSelectedTemplate] = useState<string>('');

  // Load data from API on mount
  useEffect(() => {
    const loadData = async () => {
      try {
        await loadWorkouts()
        await loadActiveSession()
        await loadTemplates()
        await loadProgressData()
      } catch (error) {
        setError('Failed to load initial data')
      }
    }
    loadData()
  }, [])

  const loadWorkouts = async () => {
    try {
      setLoading(true)
      const data = await apiService.getWorkouts()
      setWorkouts(data)
    } catch (err) {
      setError('Failed to load workouts')
    } finally {
      setLoading(false)
    }
  }

  const loadActiveSession = async () => {
    try {
      const session = await apiService.getActiveSession()
      setActiveSession(session)
    } catch (err) {
      // Silent fail for active session - it's optional
    }
  }

  const loadTemplates = async () => {
    try {
      const templatesData = await apiService.getWorkoutTemplates();
      setTemplates(templatesData);
    } catch (err) {
      console.error('Failed to load templates:', err);
    }
  };

  const createWorkout = async () => {
    if (!newWorkoutName.trim()) return
    
    try {
      setLoading(true)
      const workout = await apiService.createWorkout(newWorkoutName.trim())
      setWorkouts([...workouts, workout])
      setNewWorkoutName('')
    } catch (err) {
      setError('Failed to create workout')
    } finally {
      setLoading(false)
    }
  }

  const addExercise = async () => {
    if (!newExercise.name.trim() || !currentWorkout) return
    
    try {
      setLoading(true)
      const exercise = await apiService.createExercise({
        name: newExercise.name.trim(),
        sets: newExercise.sets,
        reps: newExercise.reps,
        weight: newExercise.weight,
        workout_id: currentWorkout.id
      })
      
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
    } catch (err) {
      setError('Failed to add exercise')
    } finally {
      setLoading(false)
    }
  }

  const addExerciseFromTemplate = async () => {
    if (!selectedTemplate || !currentWorkout) return;
    
    const template = templates.find(t => t.id === selectedTemplate);
    if (!template) return;

    setLoading(true);
    try {
      // Add all exercises from the template
      for (const exercise of template.exercises) {
        await apiService.createExercise({
          name: exercise.name,
          sets: exercise.sets,
          reps: exercise.reps,
          weight: exercise.weight,
          workout_id: currentWorkout.id
        });
      }
      
      // Refresh the current workout to show new exercises
      const updatedWorkout = await apiService.getWorkout(currentWorkout.id);
      setCurrentWorkout(updatedWorkout);
      
      // Reset template selection
      setSelectedTemplate('');
      
      // Refresh workouts list
      await loadWorkouts();
    } catch (err) {
      setError('Failed to add exercises from template');
    } finally {
      setLoading(false);
    }
  };

  const startWorkout = (workout: Workout) => {
    setCurrentWorkout(workout)
    setView('workouts')
  }

  const completeSet = async (sessionExerciseId: string, setIndex: number) => {
    if (!activeSession) return
    
    try {
      setLoading(true)
      await apiService.completeSet(sessionExerciseId, setIndex)
      loadActiveSession() // Reload active session to update completed sets
    } catch (err) {
      setError('Failed to complete set')
    } finally {
      setLoading(false)
    }
  }

  const endSession = async () => {
    if (!activeSession) return
    
    try {
      setLoading(true)
      await apiService.endSession(activeSession.id)
      loadActiveSession() // Reload active session to update its state
      loadProgressData() // Reload progress data
      setView('workouts')
    } catch (err) {
      setError('Failed to end session')
    } finally {
      setLoading(false)
    }
  }

  const loadProgressData = async () => {
    try {
      const data = await apiService.getProgressData();
      setProgressData(data);
    } catch (err) {
      console.error('Failed to load progress data:', err);
    }
  };

  const deleteWorkout = async (workoutId: string) => {
    if (window.confirm('Are you sure you want to delete this workout?')) {
      try {
        setLoading(true)
        await apiService.deleteWorkout(workoutId)
        setWorkouts(workouts.filter(w => w.id !== workoutId))
        if (currentWorkout?.id === workoutId) {
          setCurrentWorkout(null)
        }
      } catch (err) {
        setError('Failed to delete workout')
      } finally {
        setLoading(false)
      }
    }
  }

  const deleteExercise = async (exerciseId: string) => {
    if (!currentWorkout) return
    
    if (window.confirm('Are you sure you want to delete this exercise?')) {
      try {
        setLoading(true)
        await apiService.deleteExercise(exerciseId)
        const updatedWorkout = {
          ...currentWorkout,
          exercises: currentWorkout.exercises.filter(e => e.id !== exerciseId)
        }
        
        setWorkouts(workouts.map(w => w.id === currentWorkout.id ? updatedWorkout : w))
        setCurrentWorkout(updatedWorkout)
      } catch (err) {
        setError('Failed to delete exercise')
      } finally {
        setLoading(false)
      }
    }
  }

  const handleWorkoutCreated = () => {
    loadWorkouts();
    setView('workouts');
  };

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
          <button
            className={`nav-button ${view === 'library' ? 'active' : ''}`}
            onClick={() => setView('library')}
          >
            Library
          </button>
        </nav>
      </header>

      <main className="app-main">
        {error && (
          <div className="error-banner">
            <p>{error}</p>
            <button onClick={() => setError(null)}>×</button>
          </div>
        )}

        {view === 'workouts' && (
          <div className="workouts-view">
            <div className="left-panel">
              <div className="workout-section">
                <h2>Create New Workout</h2>
                <div className="input-group">
                  <input
                    type="text"
                    placeholder="Workout name..."
                    value={newWorkoutName}
                    onChange={(e) => setNewWorkoutName(e.target.value)}
                    onKeyPress={(e) => e.key === 'Enter' && createWorkout()}
                    disabled={loading}
                  />
                  <button 
                    className="btn-primary"
                    onClick={createWorkout}
                    disabled={loading || !newWorkoutName.trim()}
                  >
                    {loading ? 'Creating...' : 'Create'}
                  </button>
                </div>
              </div>

              <div className="workouts-section">
                <h2>Your Workouts</h2>
                {loading ? (
                  <div className="loading-state">
                    <p>Loading workouts...</p>
                  </div>
                ) : workouts.length === 0 ? (
                  <p className="empty-state">No workouts yet. Create your first workout above!</p>
                ) : (
                  <div className="workout-cards">
                    {workouts.map(workout => (
                      <div key={workout.id} className="workout-card">
                        <div className="workout-header">
                          <h3>{workout.name}</h3>
                          <button 
                            className="btn-delete"
                            onClick={() => deleteWorkout(workout.id)}
                            disabled={loading}
                          >
                            ×
                          </button>
                        </div>
                        <p className="workout-stats">
                          {workout.exercises?.length || 0} exercises
                        </p>
                        <div className="workout-actions">
                          <button 
                            onClick={() => startWorkout(workout)}
                            className="btn-primary"
                            disabled={loading}
                          >
                            {currentWorkout?.id === workout.id ? 'Continue' : 'Start'}
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
                  <h2>Current Workout: {currentWorkout.name}</h2>
                  <div className="add-exercise">
                    <h3>Add Exercise</h3>
                    
                    {/* Template Quick Add */}
                    <div className="template-quick-add">
                      <h4>Quick Add from Template</h4>
                      <div className="template-dropdown">
                        <select
                          value={selectedTemplate}
                          onChange={(e) => setSelectedTemplate(e.target.value)}
                          disabled={loading || templates.length === 0}
                        >
                          <option value="">Select a template...</option>
                          {templates.map(template => (
                            <option key={template.id} value={template.id}>
                              {template.name} ({template.exercises.length} exercises)
                            </option>
                          ))}
                        </select>
                        <button
                          className="btn-secondary"
                          onClick={addExerciseFromTemplate}
                          disabled={loading || !selectedTemplate}
                        >
                          Add Template Exercises
                        </button>
                      </div>
                    </div>

                    <div className="exercise-form">
                      <h4>Add Custom Exercise</h4>
                      <input
                        type="text"
                        placeholder="Exercise name..."
                        value={newExercise.name}
                        onChange={(e) => setNewExercise({...newExercise, name: e.target.value})}
                        disabled={loading}
                      />
                      <div className="exercise-inputs">
                        <input
                          type="number"
                          placeholder="Sets"
                          value={newExercise.sets}
                          onChange={(e) => setNewExercise({...newExercise, sets: parseInt(e.target.value) || 0})}
                          disabled={loading}
                        />
                        <input
                          type="number"
                          placeholder="Reps"
                          value={newExercise.reps}
                          onChange={(e) => setNewExercise({...newExercise, reps: parseInt(e.target.value) || 0})}
                          disabled={loading}
                        />
                        <input
                          type="number"
                          placeholder="Weight (lbs)"
                          value={newExercise.weight}
                          onChange={(e) => setNewExercise({...newExercise, weight: parseFloat(e.target.value) || 0})}
                          disabled={loading}
                        />
                      </div>
                      <button 
                        className="btn-primary"
                        onClick={addExercise}
                        disabled={loading || !newExercise.name.trim()}
                      >
                        {loading ? 'Adding...' : 'Add Exercise'}
                      </button>
                    </div>
                  </div>

                  <div className="exercise-cards">
                    {currentWorkout.exercises?.map(exercise => (
                      <div key={exercise.id} className="exercise-card">
                        <div className="exercise-header">
                          <h4>{exercise.name}</h4>
                          <button 
                            className="btn-delete-small"
                            onClick={() => deleteExercise(exercise.id)}
                            disabled={loading}
                          >
                            ×
                          </button>
                        </div>
                        <div className="exercise-stats">
                          <span>{`${exercise.sets} sets × ${exercise.reps} reps`}</span>
                          {exercise.weight > 0 && <span>{`${exercise.weight} lbs`}</span>}
                        </div>
                      </div>
                    )) || <p>No exercises yet</p>}
                  </div>
                </div>
              ) : (
                <div className="empty-state">
                  <p>Select a workout to add exercises</p>
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
            {loading ? (
              <p>Loading progress data...</p>
            ) : error ? (
              <p className="error-message">{error}</p>
            ) : progressData.length === 0 ? (
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

        {view === 'library' && (
          <WorkoutLibrary onWorkoutCreated={handleWorkoutCreated} />
        )}
      </main>
    </div>
  )
}