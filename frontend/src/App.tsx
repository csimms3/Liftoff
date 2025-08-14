import { useState, useEffect, useCallback, useMemo } from 'react'
import { WorkoutLibrary } from './components/WorkoutLibrary'
import { ApiService, type Workout, type WorkoutSession, type ExerciseTemplate, type ProgressData, type Exercise } from './api'
import './App.css'

/**
 * Main Application Component
 * 
 * Liftoff is a comprehensive workout tracking application that allows users to:
 * - Create and manage workout plans
 * - Track workout sessions in real-time
 * - Monitor progress over time
 * - Access exercise templates for quick workout building
 * - View workout history and analytics
 * 
 * The app uses a multi-view architecture with state management for:
 * - Workout management (create, edit, delete)
 * - Active session tracking
 * - Progress monitoring
 * - Exercise template library
 * 
 * Features:
 * - Responsive design for all screen sizes
 * - Real-time session tracking
 * - Exercise template dropdown for quick addition
 * - Progress visualization
 * - Error handling and loading states
 */
export default function App() {
  // API service instance for backend communication
  const apiService = useMemo(() => new ApiService(), [])
  
  // Application state management
  const [view, setView] = useState<'workouts' | 'session' | 'progress' | 'library'>('workouts');
  const [workouts, setWorkouts] = useState<Workout[]>([]);
  const [currentWorkout, setCurrentWorkout] = useState<Workout | null>(null);
  const [activeSession, setActiveSession] = useState<WorkoutSession | null>(null);
  const [progressData, setProgressData] = useState<ProgressData[]>([]);
  const [completedSessions, setCompletedSessions] = useState<WorkoutSession[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // Form state for creating new workouts and exercises
  const [newWorkoutName, setNewWorkoutName] = useState('')
  const [newExercise, setNewExercise] = useState({
    name: '',
    sets: 3,
    reps: 10,
    weight: 0
  })
  
  // Exercise template state for quick exercise addition
  const [exerciseTemplates, setExerciseTemplates] = useState<ExerciseTemplate[]>([]);
  const [selectedExerciseTemplate, setSelectedExerciseTemplate] = useState<string>('');

  /**
   * Load all workouts from the backend API
   * 
   * Updates the workouts state with fetched data and handles loading states.
   */
  const loadWorkouts = useCallback(async () => {
    try {
      setLoading(true)
      const data = await apiService.getWorkouts()
      
      // Load exercises for each workout
      const workoutsWithExercises = await Promise.all(
        data.map(async (workout) => {
          try {
            const exercises = await apiService.getExercisesByWorkout(workout.id)
            return { ...workout, exercises }
          } catch {
            // If loading exercises fails, use empty array
            return { ...workout, exercises: [] }
          }
        })
      )
      
      setWorkouts(workoutsWithExercises)
    } catch {
      setError('Failed to load workouts')
    } finally {
      setLoading(false)
    }
  }, [apiService])

  /**
   * Load the currently active workout session
   * 
   * Silently fails if no active session exists, as this is optional.
   */
  const loadActiveSession = useCallback(async () => {
    try {
      const session = await apiService.getActiveSession()
      setActiveSession(session)
    } catch {
      // Silent fail for active session - it's optional
    }
  }, [apiService])

  /**
   * Load exercise templates for the quick-add dropdown
   * 
   * Fetches predefined exercise templates that users can quickly add to workouts.
   */
  const loadExerciseTemplates = useCallback(async () => {
    try {
      const templatesData = await apiService.getExerciseTemplates();
      setExerciseTemplates(templatesData);
    } catch {
      console.error('Failed to load exercise templates');
    }
  }, [apiService]);

  /**
   * Load progress data for charts and analytics
   */
  const loadProgressData = useCallback(async () => {
    try {
      const data = await apiService.getProgressData();
      setProgressData(data);
    } catch {
      console.error('Failed to load progress data');
    }
  }, [apiService]);

  /**
   * Load completed workout sessions for history
   */
  const loadCompletedSessions = useCallback(async () => {
    try {
      const sessions = await apiService.getCompletedSessions();
      setCompletedSessions(sessions);
    } catch {
      console.error('Failed to load completed sessions');
    }
  }, [apiService]);

  useEffect(() => {
    const loadData = async () => {
      try {
        await Promise.all([
          loadWorkouts(),
          loadActiveSession(),
          loadExerciseTemplates(),
          loadProgressData(),
          loadCompletedSessions()
        ])
      } catch {
        setError('Failed to load initial data')
      }
    }
    loadData()
  }, [loadWorkouts, loadActiveSession, loadExerciseTemplates, loadProgressData, loadCompletedSessions])

  /**
   * Create a new workout with the specified name
   * 
   * Adds the new workout to the workouts list and clears the input field.
   */
  const createWorkout = async () => {
    if (!newWorkoutName.trim()) return
    
    try {
      setLoading(true)
      const workout = await apiService.createWorkout(newWorkoutName.trim())
      setWorkouts([...workouts, workout])
      setNewWorkoutName('')
    } catch {
      setError('Failed to create workout')
    } finally {
      setLoading(false)
    }
  }

  /**
   * Add a new exercise to the current workout
   * 
   * Creates the exercise via API and updates the current workout state.
   */
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
        exercises: [...(currentWorkout.exercises || []), exercise]
      }
      
      setWorkouts(workouts.map((w: Workout) => w.id === currentWorkout.id ? updatedWorkout : w))
      setCurrentWorkout(updatedWorkout)
      
      setNewExercise({
        name: '',
        sets: 3,
        reps: 10,
        weight: 0
      })
    } catch (error) {
      console.error('Exercise creation error:', error);
      setError('Failed to add exercise')
    } finally {
      setLoading(false)
    }
  }

  const addExerciseFromTemplate = async () => {
    if (!selectedExerciseTemplate || !currentWorkout) return;
    
    const template = exerciseTemplates.find((t: ExerciseTemplate) => t.name === selectedExerciseTemplate);
    if (!template) return;

    setLoading(true);
    try {
      // Add the exercise from the template
      const newExercise = await apiService.createExercise({
        name: template.name,
        sets: template.default_sets,
        reps: template.default_reps,
        weight: template.default_weight,
        workout_id: currentWorkout.id
      });
      
      // Update the current workout locally with the new exercise
      const updatedWorkout = {
        ...currentWorkout,
        exercises: [...(currentWorkout.exercises || []), newExercise]
      };
      setCurrentWorkout(updatedWorkout);
      
      // Update the workouts list locally
      setWorkouts(workouts.map((w: Workout) => 
        w.id === currentWorkout.id ? updatedWorkout : w
      ));
      
      // Reset template selection
      setSelectedExerciseTemplate('');
    } catch (error) {
      console.error('Template exercise creation error:', error);
      setError('Failed to add exercise from template');
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
    } catch {
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
    } catch {
      setError('Failed to end session')
    } finally {
      setLoading(false)
    }
  }

  const deleteWorkout = async (workoutId: string) => {
    if (window.confirm('Are you sure you want to delete this workout?')) {
      try {
        setLoading(true)
        await apiService.deleteWorkout(workoutId)
        setWorkouts(workouts.filter((w: Workout) => w.id !== workoutId))
        if (currentWorkout?.id === workoutId) {
          setCurrentWorkout(null)
        }
      } catch {
        setError('Failed to delete workout')
      } finally {
        setLoading(false)
      }
    }
  }

  const deleteExercise = async (exerciseId: string) => {
    if (!currentWorkout) return
    
    try {
      setLoading(true)
      await apiService.deleteExercise(exerciseId)
      const updatedWorkout = {
        ...currentWorkout,
        exercises: currentWorkout.exercises.filter((e: Exercise) => e.id !== exerciseId)
      }
      
      setWorkouts(workouts.map((w: Workout) => w.id === currentWorkout.id ? updatedWorkout : w))
      setCurrentWorkout(updatedWorkout)
    } catch {
      setError('Failed to delete exercise')
    } finally {
      setLoading(false)
    }
  }

  const handleWorkoutCreated = () => {
    loadWorkouts();
    setView('workouts');
  };

  /**
   * Add an exercise from the library to the current workout
   * 
   * Creates a new exercise using the template data and adds it to the current workout.
   */
  const addExerciseFromLibrary = async (template: ExerciseTemplate) => {
    if (!currentWorkout) {
      setError('Please select a workout first');
      return;
    }
    
    try {
      setLoading(true);
      const exercise = await apiService.createExercise({
        name: template.name,
        sets: template.default_sets,
        reps: template.default_reps,
        weight: template.default_weight,
        workout_id: currentWorkout.id
      });
      
      // Update the current workout with the new exercise
      const updatedWorkout = {
        ...currentWorkout,
        exercises: [...(currentWorkout.exercises || []), exercise]
      };
      
      // Update both the workouts list and current workout
      setWorkouts(workouts.map((w: Workout) => w.id === currentWorkout.id ? updatedWorkout : w));
      setCurrentWorkout(updatedWorkout);
      
      // Switch to workouts view to show the updated workout
      setView('workouts');
    } catch {
      setError('Failed to add exercise from library');
    } finally {
      setLoading(false);
    }
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
                          {workout.exercises?.length || 0} {(workout.exercises?.length || 0) === 1 ? 'exercise' : 'exercises'}
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
                      <h4>Quick Add Exercise</h4>
                      <div className="template-dropdown">
                        <select
                          value={selectedExerciseTemplate}
                          onChange={(e) => setSelectedExerciseTemplate(e.target.value)}
                          disabled={loading || exerciseTemplates.length === 0}
                        >
                          <option value="">Select an exercise...</option>
                          {exerciseTemplates.map(template => (
                            <option key={template.name} value={template.name}>
                              {template.name} ({template.category})
                            </option>
                          ))}
                        </select>
                        <button
                          className="btn-secondary"
                          onClick={addExerciseFromTemplate}
                          disabled={loading || !selectedExerciseTemplate}
                        >
                          Add Exercise
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
                <span>Started: {new Date(activeSession.started_at).toLocaleTimeString()}</span>
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
            
            {/* Progress Charts Section */}
            <div className="progress-section">
              <h3>Exercise Progress</h3>
              {loading ? (
                <p>Loading progress data...</p>
              ) : error ? (
                <p className="error-message">{error}</p>
              ) : !progressData || progressData.length === 0 ? (
                <p className="empty-state">No progress data yet. Complete some workouts to see your progress!</p>
              ) : (
                <div className="progress-charts">
                  <div className="progress-summary">
                    <h4>Recent Activity</h4>
                    <div className="progress-cards">
                      {progressData.slice(-5).reverse().map((data, index) => (
                        <div key={index} className="progress-card">
                          <h5>{data.exerciseName}</h5>
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

            {/* Workout History Section */}
            <div className="workout-history-section">
              <h3>Workout History</h3>
              {loading ? (
                <p>Loading workout history...</p>
              ) : !completedSessions || completedSessions.length === 0 ? (
                <p className="empty-state">No completed workouts yet. Finish a workout to see it here!</p>
              ) : (
                <div className="workout-history-list">
                  {completedSessions.map((session) => (
                    <div key={session.id} className="workout-history-card">
                      <div className="workout-history-header">
                        <h4>{session.workout?.name || 'Unknown Workout'}</h4>
                        <span className="workout-date">
                          {new Date(session.started_at).toLocaleDateString()}
                        </span>
                      </div>
                      <div className="workout-history-details">
                        <span>Started: {new Date(session.started_at).toLocaleTimeString()}</span>
                        <span>Duration: {session.ended_at ? 
                          Math.round((new Date(session.ended_at).getTime() - new Date(session.started_at).getTime()) / 60000) + ' min' : 
                          'Unknown'
                        }</span>
                        <span>Exercises: {session.exercises?.length || 0}</span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {view === 'library' && (
          <WorkoutLibrary 
            onWorkoutCreated={handleWorkoutCreated}
            onExerciseSelected={addExerciseFromLibrary}
          />
        )}
      </main>
    </div>
  )
}