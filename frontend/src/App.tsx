import { useState, useEffect, useCallback, useMemo } from 'react'
import { WorkoutLibrary } from './components/WorkoutLibrary'
import { SetLoggingForm } from './components/SetLoggingForm'
import { QuickLogSetForm } from './components/QuickLogSetForm'
import { DinoGame } from './components/DinoGame'
import { ApiService, type Workout, type WorkoutSession, type ExerciseTemplate, type ProgressData, type Exercise, type ExerciseSet } from './api'
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

  // Settings menu state
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  
  // Dino game state
  const [isDinoGameOpen, setIsDinoGameOpen] = useState(false);
  
  // Theme state
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    const savedTheme = localStorage.getItem('liftoff-theme');
    return (savedTheme as 'light' | 'dark') || 'light';
  });

  // Weight unit state
  const [weightUnit, setWeightUnit] = useState<'lbs' | 'kg'>(() => {
    const savedUnit = localStorage.getItem('liftoff-weight-unit');
    return (savedUnit as 'lbs' | 'kg') || 'lbs';
  });
  
  /**
   * Apply theme to document body and save to localStorage
   */
  useEffect(() => {
    document.body.setAttribute('data-theme', theme);
    localStorage.setItem('liftoff-theme', theme);
  }, [theme]);

  /**
   * Save weight unit to localStorage
   */
  useEffect(() => {
    localStorage.setItem('liftoff-weight-unit', weightUnit);
  }, [weightUnit]);

  /**
   * Convert weight between units
   */
  const convertWeight = (weight: number, fromUnit: 'lbs' | 'kg', toUnit: 'lbs' | 'kg'): number => {
    if (fromUnit === toUnit) return weight;
    if (fromUnit === 'lbs' && toUnit === 'kg') return weight * 0.453592;
    if (fromUnit === 'kg' && toUnit === 'lbs') return weight * 2.20462;
    return weight;
  };

  /**
   * Format weight with unit
   */
  const formatWeight = (weight: number): string => {
    const convertedWeight = convertWeight(weight, 'lbs', weightUnit);
    return `${convertedWeight.toFixed(1)} ${weightUnit}`;
  };

  /**
   * Determines the weight type for an exercise based on its name
   */
  const getWeightType = (exerciseName: string): string => {
    const name = exerciseName.toLowerCase();
    
    // Bodyweight exercises
    const bodyweightKeywords = [
      'push-up', 'pull-up', 'chin-up', 'dip', 'plank', 'crunch', 'sit-up',
      'lunge', 'burpee', 'mountain climber', 'jump squat', 'high knee',
      'side plank', 'russian twist', 'leg raise', 'pike', 'bear crawl',
      'wall sit', 'jumping jack', 'squat jump', 'pistol squat', 'handstand'
    ];
    
    // Machine-based exercises
    const machineKeywords = [
      'lat pulldown', 'cable', 'machine', 'leg press', 'chest press',
      'seated row', 'tricep pushdown', 'leg extension', 'leg curl',
      'chest fly', 'shoulder press machine', 'ab crunch machine'
    ];
    
    // Check for bodyweight exercises
    if (bodyweightKeywords.some(keyword => name.includes(keyword))) {
      return 'Bodyweight';
    }
    
    // Check for machine exercises
    if (machineKeywords.some(keyword => name.includes(keyword))) {
      return 'Machine';
    }
    
    // Check for weighted exercises (barbell, dumbbell, kettlebell, etc.)
    const weightedKeywords = [
      'barbell', 'dumbbell', 'kettlebell', 'weighted', 'deadlift',
      'squat', 'press', 'row', 'curl', 'extension', 'raise', 'fly'
    ];
    
    if (weightedKeywords.some(keyword => name.includes(keyword))) {
      return 'Weighted';
    }
    
    // Default to "Weighted" for exercises that don't match bodyweight patterns
    return 'Weighted';
  };

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
      console.log('Loaded active session:', session)
      setActiveSession(session)
      return session
    } catch (error) {
      console.log('No active session or error:', error)
      // Silent fail for active session - it's optional
      return null
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

  // Debug active session changes
  useEffect(() => {
    if (activeSession) {
      console.log('Active session changed:', activeSession)
      console.log('Exercises:', activeSession.exercises)
      console.log('Workout:', activeSession.workout)
    }
  }, [activeSession])

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

  const startWorkout = async (workout: Workout) => {
    try {
      setLoading(true)
      console.log('Starting workout:', workout)
      
      // Create a new session for this workout
      const session = await apiService.createSession(workout.id)
      console.log('Session created:', session)
      setActiveSession(session)
      setCurrentWorkout(workout)
      
      // Refresh the active session to ensure exercises are loaded
      const refreshedSession = await loadActiveSession()
      console.log('Refreshed session:', refreshedSession)
      
      // Don't auto-switch views - let user click Active Session tab
    } catch (error) {
      console.error('Failed to start workout session:', error)
      setError('Failed to start workout session')
    } finally {
      setLoading(false)
    }
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

  const logSet = async (setId: string, reps: number, weight: number, notes?: string) => {
    try {
      setLoading(true)
      await apiService.updateSet(setId, reps, weight, notes)
      loadActiveSession() // Reload active session to update logged sets
    } catch (error) {
      console.error('Failed to log set:', error)
      setError('Failed to log set')
    } finally {
      setLoading(false)
    }
  }

  const quickLogSet = async (exerciseId: string, reps: number, weight: number, notes?: string) => {
    try {
      setLoading(true)
      // Create a temporary session exercise and set for logging
      const session = await apiService.createSession(currentWorkout!.id)
      const sessionExercise = await apiService.addExerciseToSession(session.id, exerciseId)
      const set = await apiService.createSet(sessionExercise.id, reps, weight)
      await apiService.updateSet(set.id, reps, weight, notes)
      // End the session immediately after logging
      await apiService.endSession(session.id)
      loadProgressData() // Refresh progress data
    } catch (error) {
      console.error('Failed to quick log set:', error)
      setError('Failed to log set')
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
        <div className="header-content">
          <div className="header-left">
            <h1 style={{ cursor: 'pointer' }} onClick={() => setIsDinoGameOpen(true)}>🏋️ Liftoff</h1>
            <p>Track your workouts and build strength</p>
          </div>
          <div className="header-controls">
            <button 
              className="theme-toggle"
              onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')}
              aria-label={`Switch to ${theme === 'light' ? 'dark' : 'light'} theme`}
            >
              {theme === 'light' ? '🌙' : '☀️'}
            </button>
            <button 
              className="hamburger-menu"
              onClick={() => setIsSettingsOpen(!isSettingsOpen)}
              aria-label="Settings menu"
            >
              <span></span>
              <span></span>
              <span></span>
            </button>
          </div>
        </div>
        
        {isSettingsOpen && (
          <div className="settings-menu">
            <div className="settings-header">
              <h3>Settings</h3>
              <button 
                className="close-settings"
                onClick={() => setIsSettingsOpen(false)}
                aria-label="Close settings"
              >
                ×
              </button>
            </div>
            <div className="settings-content">
              <div className="setting-item">
                <label>Theme</label>
                <select 
                  value={theme} 
                  onChange={(e) => setTheme(e.target.value as 'light' | 'dark')}
                >
                  <option value="light">Light</option>
                  <option value="dark">Dark</option>
                </select>
              </div>
              <div className="setting-item">
                <label>Units</label>
                <select 
                  value={weightUnit} 
                  onChange={(e) => setWeightUnit(e.target.value as 'lbs' | 'kg')}
                >
                  <option value="lbs">Pounds (lbs)</option>
                  <option value="kg">Kilograms (kg)</option>
                </select>
              </div>
              <div className="setting-item">
                <label>Notifications</label>
                <input type="checkbox" defaultChecked />
              </div>
              <div className="setting-item">
                <label>Auto-save</label>
                <input type="checkbox" defaultChecked />
              </div>
            </div>
          </div>
        )}

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
                            {activeSession && activeSession.workout.id === workout.id ? 'Continue Session' : 'Start'}
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
                              {template.name} ({template.category}) - {getWeightType(template.name)}
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
                        <div className="input-group">
                          <label htmlFor="sets-input">Sets</label>
                          <input
                            id="sets-input"
                            type="number"
                            placeholder="Number of sets"
                            value={newExercise.sets}
                            onChange={(e) => setNewExercise({...newExercise, sets: parseInt(e.target.value) || 0})}
                            disabled={loading}
                          />
                        </div>
                        <div className="input-group">
                          <label htmlFor="reps-input">Reps</label>
                          <input
                            id="reps-input"
                            type="number"
                            placeholder="Reps per set"
                            value={newExercise.reps}
                            onChange={(e) => setNewExercise({...newExercise, reps: parseInt(e.target.value) || 0})}
                            disabled={loading}
                          />
                        </div>
                        <div className="input-group">
                          <label htmlFor="weight-input">Weight ({weightUnit})</label>
                          <input
                            id="weight-input"
                            type="number"
                            placeholder={`Weight in ${weightUnit}`}
                            value={newExercise.weight}
                            onChange={(e) => setNewExercise({...newExercise, weight: parseFloat(e.target.value) || 0})}
                            disabled={loading}
                          />
                        </div>
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
                          {exercise.weight > 0 && <span>{formatWeight(exercise.weight)}</span>}
                        </div>
                        <QuickLogSetForm
                          exerciseName={exercise.name}
                          plannedReps={exercise.reps}
                          plannedWeight={exercise.weight}
                          onLogSet={(reps, weight, notes) => quickLogSet(exercise.id, reps, weight, notes)}
                          loading={loading}
                          weightUnit={weightUnit}
                        />
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
          <div className="workouts-view">
            <div className="left-panel">
              <div className="current-workout">
                <h2>Active Session: {activeSession.workout?.name}</h2>
                <div className="session-info">
                  <p className="workout-stats">
                    Started: {new Date(activeSession.started_at).toLocaleTimeString()}
                  </p>
                  <div className="workout-actions">
                    <button onClick={endSession} className="btn-danger">
                      End Session
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <div className="right-panel">
              <div className="workouts-section">
                <h2>Session Exercises</h2>
                {activeSession.exercises?.length > 0 ? (
                  <div className="exercise-cards">
                    {activeSession.exercises.map(sessionExercise => (
                      <div key={sessionExercise.id} className="exercise-card">
                        <div className="exercise-header">
                          <h4>{sessionExercise.exercise?.name}</h4>
                        </div>
                        <div className="exercise-stats">
                          {sessionExercise.sets?.length > 0 ? (
                            <div className="sets-grid">
                              {sessionExercise.sets.map((set, index) => (
                                <SetLoggingForm
                                  key={set.id}
                                  set={set}
                                  setIndex={index}
                                  onLogSet={logSet}
                                  loading={loading}
                                />
                              ))}
                            </div>
                          ) : (
                            <p className="empty-state">No sets available</p>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="empty-state">No exercises in this session</p>
                )}
              </div>
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
                            <span>Max Weight: {formatWeight(data.maxWeight)}</span>
                            <span>Volume: {formatWeight(data.totalVolume)}</span>
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
            onExerciseSelected={addExerciseFromLibrary}
          />
        )}
      </main>

      <footer className="app-footer">
        <div className="footer-content">
          <div className="footer-section">
            <h4>🏋️ Liftoff</h4>
            <p>Track your workouts and build strength</p>
          </div>
          <div className="footer-section">
            <h4>Features</h4>
            <ul>
              <li>Workout Management</li>
              <li>Progress Tracking</li>
              <li>Exercise Library</li>
              <li>Session Tracking</li>
            </ul>
          </div>
          <div className="footer-section">
            <h4>Quick Links</h4>
            <ul>
              <li><button onClick={() => setView('workouts')}>Workouts</button></li>
              <li><button onClick={() => setView('progress')}>Progress</button></li>
              <li><button onClick={() => setView('library')}>Library</button></li>
            </ul>
          </div>
        </div>
        <div className="footer-bottom">
          <p>&copy; 2025 Liftoff. Built with React & Go.</p>
        </div>
      </footer>

      <DinoGame isOpen={isDinoGameOpen} onClose={() => setIsDinoGameOpen(false)} />
    </div>
  )
}