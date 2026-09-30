import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { WorkoutLibrary } from './components/WorkoutLibrary'
import { SetTable } from './components/SetTable'
import { ExercisePicker, type PickTarget } from './components/ExercisePicker'
import { ExerciseMenu } from './components/ExerciseMenu'
import { FinishDialog } from './components/FinishDialog'
import { RestTimerBar } from './components/RestTimerBar'
import { RestSettingDialog } from './components/RestSettingDialog'
import { SessionHeader } from './components/SessionHeader'
import { formatRest } from './formatRest'
import { useRestTimer } from './useRestTimer'
import { PlannedExerciseCard } from './components/PlannedExerciseCard'
import { DinoGame } from './components/DinoGame'
import { useAuth } from './context/useAuth'
import { ApiService, type Workout, type WorkoutSession, type ExerciseTemplate, type ProgressData, type Exercise, type ExerciseSet, type SessionExercise, type Routine, type RoutineTemplate } from './api'
import './App.css'

export default function App() {
  const { user, logout, sessionTimeoutMinutes, setSessionTimeoutMinutes, isAdmin, setShowAdmin, setIdleLogoutPaused } = useAuth()
  const apiService = useMemo(() => new ApiService(), [])
  
  const [view, setView] = useState<'workouts' | 'routines' | 'session' | 'progress' | 'library'>('workouts');
  const [workouts, setWorkouts] = useState<Workout[]>([]);
  const [currentWorkout, setCurrentWorkout] = useState<Workout | null>(null);
  const [activeSession, setActiveSession] = useState<WorkoutSession | null>(null);
  const [progressData, setProgressData] = useState<ProgressData[]>([]);
  const [completedSessions, setCompletedSessions] = useState<WorkoutSession[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [progressError, setProgressError] = useState<string | null>(null);
  
  const [newWorkoutName, setNewWorkoutName] = useState('')
  const [addingWorkout, setAddingWorkout] = useState(false)
  const [exerciseTemplates, setExerciseTemplates] = useState<ExerciseTemplate[]>([]);
  const [addingToWorkout, setAddingToWorkout] = useState(false);
  const [routines, setRoutines] = useState<Routine[]>([]);
  const [currentRoutineId, setCurrentRoutineId] = useState<string | null>(null);
  const [routineTemplates, setRoutineTemplates] = useState<RoutineTemplate[]>([]);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isDinoGameOpen, setIsDinoGameOpen] = useState(false);
  
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    const savedTheme = localStorage.getItem('liftoff-theme');
    return (savedTheme as 'light' | 'dark') || 'light';
  });

  // Weight unit state
  const [weightUnit, setWeightUnit] = useState<'lbs' | 'kg'>(() => {
    const savedUnit = localStorage.getItem('liftoff-weight-unit');
    return (savedUnit as 'lbs' | 'kg') || 'lbs';
  });
  
  // Don't log out for inactivity in the middle of a workout.
  useEffect(() => {
    setIdleLogoutPaused(!!activeSession)
    return () => setIdleLogoutPaused(false)
  }, [activeSession, setIdleLogoutPaused])

  useEffect(() => {
    document.body.setAttribute('data-theme', theme);
    localStorage.setItem('liftoff-theme', theme);
  }, [theme]);

  useEffect(() => {
    localStorage.setItem('liftoff-weight-unit', weightUnit);
  }, [weightUnit]);

  const convertWeight = (weight: number, fromUnit: 'lbs' | 'kg', toUnit: 'lbs' | 'kg'): number => {
    if (fromUnit === toUnit) return weight;
    if (fromUnit === 'lbs' && toUnit === 'kg') return weight * 0.453592;
    if (fromUnit === 'kg' && toUnit === 'lbs') return weight * 2.20462;
    return weight;
  };

  const formatWeight = (weight: number): string => {
    const convertedWeight = convertWeight(weight, 'lbs', weightUnit);
    return `${convertedWeight.toFixed(1)} ${weightUnit}`;
  };

  const loadWorkouts = useCallback(async () => {
    try {
      setLoading(true)
      const data = await apiService.getWorkouts()
      const list = Array.isArray(data) ? data : []

      const workoutsWithExercises = await Promise.all(
        list.map(async (workout) => {
          try {
            const exercises = await apiService.getExercisesByWorkout(workout.id)
            return { ...workout, exercises }
          } catch {
            return { ...workout, exercises: [] }
          }
        })
      )
      
      setWorkouts(workoutsWithExercises)
    } catch (err) {
      console.error('Failed to load workouts:', err)
      setError('Failed to load workouts')
    } finally {
      setLoading(false)
    }
  }, [apiService])

  // Keeps App's copy of the session in step with edits made in the set rows, which
  // is what the rows are rebuilt from after switching views.
  const updateSessionSets = useCallback(
    (sessionExerciseId: string, update: (sets: ExerciseSet[]) => ExerciseSet[]) => {
      setActiveSession(s => s && {
        ...s,
        exercises: s.exercises.map(se => (se.id === sessionExerciseId ? { ...se, sets: update(se.sets) } : se)),
      })
    },
    [],
  )

  const loadActiveSession = useCallback(async () => {
    try {
      const session = await apiService.getActiveSession()
      setActiveSession(session)
      return session
    } catch {
      return null
    }
  }, [apiService])

  // Mid-session exercise edits. The server does the work; the session here is
  // updated from its reply (a reply for a session that has since ended is ignored).
  const moveQueue = useRef<Promise<void>>(Promise.resolve())
  const restTimer = useRestTimer(activeSession?.id)
  const [restFor, setRestFor] = useState<string | null>(null) // session exercise id whose rest setting is open

  // Remember an exercise's rest time (every workout that uses it), then reflect it here.
  const setRestSeconds = async (se: SessionExercise, seconds: number) => {
    setRestFor(null)
    if (!activeSession) return
    const sessionId = activeSession.id
    const apply = (n: number) =>
      editSessionExercises(sessionId, xs => xs.map(x => (x.movement_id === se.movement_id ? { ...x, rest_seconds: n } : x)))
    apply(seconds) // optimistic
    try {
      await apiService.setMovementRest(se.movement_id, seconds)
    } catch {
      apply(se.rest_seconds)
      setError('Failed to save rest time')
    }
  }

  const [picker, setPicker] = useState<{ mode: 'add' } | { mode: 'replace'; id: string } | null>(null)

  const editSessionExercises = (sessionId: string, change: (exercises: SessionExercise[]) => SessionExercise[]) =>
    setActiveSession(s => (s && s.id === sessionId ? { ...s, exercises: change(s.exercises) } : s))

  const addSessionExercise = async (target: PickTarget) => {
    if (!activeSession) return
    setPicker(null)
    try {
      const added = await apiService.addSessionExercise(activeSession.id, target)
      editSessionExercises(activeSession.id, xs => [...xs, added])
      setTimeout(() => document.getElementById(`se-${added.id}`)?.scrollIntoView({ behavior: 'smooth', block: 'center' }), 50)
    } catch {
      setError('Failed to add exercise')
    }
  }

  const replaceSessionExercise = async (id: string, target: PickTarget) => {
    if (!activeSession) return
    setPicker(null)
    const current = activeSession.exercises.find(x => x.id === id)
    const logged = current?.sets.filter(x => x.completed).length ?? 0
    if (logged > 0 && !window.confirm(`Replace ${current?.exercise?.name ?? current?.name}? Its ${logged} logged ${logged === 1 ? 'set' : 'sets'} will count as the new exercise.`)) return
    try {
      const replaced = await apiService.replaceSessionExercise(id, target)
      editSessionExercises(activeSession.id, xs => xs.map(x => (x.id === id ? replaced : x)))
    } catch {
      setError('Failed to replace exercise')
    }
  }

  const removeSessionExercise = async (se: SessionExercise) => {
    if (!activeSession) return
    const logged = se.sets.filter(x => x.completed).length
    if (logged > 0 && !window.confirm(`Remove ${se.exercise?.name ?? se.name}? Its ${logged} logged ${logged === 1 ? 'set' : 'sets'} will be deleted.`)) return
    try {
      await apiService.removeSessionExercise(se.id)
      editSessionExercises(activeSession.id, xs => xs.filter(x => x.id !== se.id))
    } catch {
      setError('Failed to remove exercise')
    }
  }

  const moveSessionExercise = async (se: SessionExercise, delta: number) => {
    if (!activeSession) return
    const from = activeSession.exercises.findIndex(x => x.id === se.id)
    const to = from + delta
    if (from < 0 || to < 0 || to >= activeSession.exercises.length) return
    editSessionExercises(activeSession.id, xs => {
      const next = [...xs]
      ;[next[from], next[to]] = [next[to], next[from]]
      return next
    }) // optimistic
    // One move at a time, so quick taps reach the server in order.
    moveQueue.current = moveQueue.current.then(async () => {
      try {
        await apiService.moveSessionExercise(se.id, to)
      } catch {
        setError('Failed to move exercise')
        loadActiveSession() // back to what the server has
      }
    })
  }

  const loadExerciseTemplates = useCallback(async () => {
    try {
      const templatesData = await apiService.getExerciseTemplates();
      setExerciseTemplates(templatesData);
    } catch {
      console.error('Failed to load exercise templates');
    }
  }, [apiService]);

  const loadProgressData = useCallback(async () => {
    try {
      const data = await apiService.getProgressData();
      setProgressData(data);
      setProgressError(null);
    } catch {
      console.error('Failed to load progress data');
      setProgressError('Failed to load progress data');
    }
  }, [apiService]);

  const loadCompletedSessions = useCallback(async () => {
    try {
      const sessions = await apiService.getCompletedSessions();
      setCompletedSessions(sessions);
    } catch {
      console.error('Failed to load completed sessions');
    }
  }, [apiService]);

  const loadRoutines = useCallback(async () => {
    try {
      const data = await apiService.getRoutines();
      setRoutines(Array.isArray(data) ? data : []);
    } catch {
      setRoutines([]);
    }
  }, [apiService]);

  const loadCurrentRoutine = useCallback(async () => {
    try {
      setCurrentRoutineId(await apiService.getCurrentRoutineId() ?? null);
    } catch {
      setCurrentRoutineId(null);
    }
  }, [apiService]);

  const loadRoutineTemplates = useCallback(async () => {
    try {
      const data = await apiService.getRoutineTemplates();
      setRoutineTemplates(Array.isArray(data) ? data : []);
    } catch {
      setRoutineTemplates([]);
    }
  }, [apiService]);

  // An error belongs to the tab it happened on, and a toast shouldn't linger.
  useEffect(() => { setError(null) }, [view])
  // Retry a failed progress load when the tab is opened; its inline error stays until one succeeds.
  useEffect(() => { if (view === 'progress') loadProgressData() }, [view, loadProgressData])
  useEffect(() => {
    if (!error) return
    const t = setTimeout(() => setError(null), 6000)
    return () => clearTimeout(t)
  }, [error])

  useEffect(() => {
    const loadData = async () => {
      try {
        await Promise.all([
          loadWorkouts(),
          loadRoutines(),
          loadCurrentRoutine(),
          loadRoutineTemplates(),
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
  }, [loadWorkouts, loadRoutines, loadCurrentRoutine, loadRoutineTemplates, loadActiveSession, loadExerciseTemplates, loadProgressData, loadCompletedSessions])

  const currentRoutine = routines.find(r => r.id === currentRoutineId)
  // The server orders workouts by routine, then position.
  const routineWorkouts = workouts.filter(w => w.routine_id === currentRoutineId)

  const cancelAddWorkout = () => {
    setAddingWorkout(false)
    setNewWorkoutName('')
  }

  // Adds a workout to the current routine.
  const createWorkout = async () => {
    if (!newWorkoutName.trim()) return
    
    try {
      setLoading(true)
      await apiService.createWorkout(newWorkoutName.trim(), currentRoutineId ?? undefined)
      setNewWorkoutName('')
      setAddingWorkout(false)
      // It lands at the end of the current routine. With no routine yet, the server starts "My Workouts".
      await Promise.all([loadWorkouts(), loadRoutines(), loadCurrentRoutine()])
    } catch {
      setError('Failed to create workout')
    } finally {
      setLoading(false)
    }
  }

  // Adds to the selected workout's plan: library defaults if the name matches one, else 3 x 10.
  const addPlannedExercise = async (name: string) => {
    if (!currentWorkout) return
    setAddingToWorkout(false)
    const template = exerciseTemplates.find((t: ExerciseTemplate) => t.name.toLowerCase() === name.toLowerCase())
    try {
      setLoading(true)
      const exercise = await apiService.createExercise({
        name,
        sets: template?.default_sets ?? 3,
        reps: template?.default_reps ?? 10,
        weight: template?.default_weight ?? 0,
        workout_id: currentWorkout.id
      })
      updateWorkoutExercises(currentWorkout.id, list => [...list, exercise])
    } catch (error) {
      console.error('Exercise creation error:', error);
      setError('Failed to add exercise')
    } finally {
      setLoading(false)
    }
  }

  // Applies fn to the latest exercises of a workout, so results that arrive after an await never clobber newer changes.
  const updateWorkoutExercises = (workoutId: string, fn: (exercises: Exercise[]) => Exercise[]) => {
    const apply = (w: Workout) => w.id === workoutId ? { ...w, exercises: fn(w.exercises || []) } : w
    setWorkouts(ws => ws.map(apply))
    setCurrentWorkout(w => w && apply(w))
  }

  const savePlannedExercise = async (exercise: Exercise, patch: { sets?: number; reps?: number; weight?: number }) => {
    try {
      const saved = await apiService.updateExercise(exercise.id, patch)
      updateWorkoutExercises(exercise.workout_id, list => list.map(e => e.id === saved.id ? saved : e))
    } catch {
      setError('Failed to update exercise')
      throw new Error('update failed') // keeps the card open for another try
    }
  }

  // One move at a time, so the local order always matches what the server did.
  const [moving, setMoving] = useState(false)
  const movePlannedExercise = async (exercise: Exercise, delta: -1 | 1) => {
    if (!currentWorkout || moving) return
    const from = currentWorkout.exercises.findIndex((e: Exercise) => e.id === exercise.id)
    const to = from + delta
    if (from < 0 || to < 0 || to >= currentWorkout.exercises.length) return
    setMoving(true)
    try {
      await apiService.updateExercise(exercise.id, { position: to })
      updateWorkoutExercises(exercise.workout_id, list => {
        const next = [...list]
        const i = next.findIndex(e => e.id === exercise.id)
        if (i >= 0) next.splice(to, 0, next.splice(i, 1)[0])
        return next
      })
    } catch {
      setError('Failed to move exercise')
    } finally {
      setMoving(false)
    }
  }

  const startWorkout = async (workout: Workout) => {
    // Continuing the workout in progress: just show it. Starting any session ends
    // the active one on the server.
    if (activeSession?.workout_id === workout.id) {
      setCurrentWorkout(workout)
      setView('session')
      return
    }
    if (activeSession && !window.confirm(`Starting "${workout.name}" will end your current workout. Continue?`)) {
      return
    }
    try {
      setLoading(true)
      const session = await apiService.createSession(workout.id)
      setActiveSession(session)
      setCurrentWorkout(workout)
      await loadActiveSession()
      setView('session')
    } catch (error) {
      console.error('Failed to start workout session:', error)
      setError('Failed to start workout session')
    } finally {
      setLoading(false)
    }
  }

  const [finishing, setFinishing] = useState(false)
  const [finishError, setFinishError] = useState<string | null>(null)

  // A failed Finish/Discard is shown in the dialog. If the session has changed
  // underneath it (ended elsewhere: 404; sets logged elsewhere: 409), the dialog's
  // summary is stale, so close it and reload what the server has.
  const finishFailed = async (err: unknown, what: string) => {
    const status = err instanceof Error ? /status: (\d+)/.exec(err.message)?.[1] : undefined
    if (status === '404' || status === '409') {
      setFinishing(false)
      setError(status === '404' ? 'That workout has already ended' : 'Sets were logged in this workout, so it can\'t be discarded')
      await loadActiveSession()
      return
    }
    setFinishError(`Couldn't ${what}. Try again.`)
  }

  // Ends the session from the Finish dialog, optionally saving its structure to
  // the workout, or discards it (nothing logged).
  const finishSession = async (updateWorkout: boolean) => {
    if (!activeSession) return
    try {
      setLoading(true)
      setFinishError(null)
      await apiService.finishSession(activeSession.id, updateWorkout)
      setFinishing(false)
      await loadActiveSession()
      loadProgressData()
      if (updateWorkout) loadWorkouts() // the workout changed
      setView('workouts')
    } catch (err) {
      await finishFailed(err, 'finish the workout')
    } finally {
      setLoading(false)
    }
  }

  const discardSession = async () => {
    if (!activeSession) return
    try {
      setLoading(true)
      setFinishError(null)
      await apiService.discardSession(activeSession.id)
      setFinishing(false)
      await loadActiveSession()
      setView('workouts')
    } catch (err) {
      await finishFailed(err, 'discard the workout')
    } finally {
      setLoading(false)
    }
  }

  // Clicking a workout card opens its plan in the editor; on narrow screens the editor is below the list.
  const editorPanel = useRef<HTMLDivElement>(null)
  const selectWorkout = (workout: Workout) => {
    setCurrentWorkout(workout)
    setTimeout(() => editorPanel.current?.scrollIntoView?.({ behavior: 'smooth', block: 'start' }), 0)
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

  const createRoutineFromTemplate = async (templateId: string, name?: string) => {
    try {
      setLoading(true)
      await apiService.createRoutineFromTemplate(templateId, name)
      // The server makes it current only if it's the user's first routine.
      await Promise.all([loadRoutines(), loadWorkouts(), loadCurrentRoutine()])
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create routine')
    } finally {
      setLoading(false)
    }
  }

  const switchRoutine = async (routineId: string) => {
    if (routineId === currentRoutineId) return
    try {
      await apiService.setCurrentRoutine(routineId)
      setCurrentRoutineId(routineId)
      if (currentWorkout && currentWorkout.routine_id !== routineId) setCurrentWorkout(null)
    } catch {
      setError('Failed to switch routine')
    }
  }

  const deleteRoutine = async (routineId: string) => {
    const routine = routines.find(r => r.id === routineId)
    const count = routine?.workouts?.length ?? 0
    const what = count === 0
      ? 'It has no workouts.'
      : `Its ${count} ${count === 1 ? 'workout' : 'workouts'} will be deleted too.`
    if (window.confirm(`Delete the routine "${routine?.name ?? 'this routine'}"? ${what} Logged workout history is kept.`)) {
      try {
        setLoading(true)
        await apiService.deleteRoutine(routineId)
        setRoutines(routines.filter(r => r.id !== routineId))
        // Its workouts went with it.
        if (currentWorkout?.routine_id === routineId) setCurrentWorkout(null)
        // If it was the current one, the server picked another.
        await Promise.all([loadWorkouts(), loadCurrentRoutine()])
      } catch {
        setError('Failed to delete routine')
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
      updateWorkoutExercises(currentWorkout.id, list => list.filter(e => e.id !== exerciseId))
    } catch {
      setError('Failed to delete exercise')
    } finally {
      setLoading(false)
    }
  }

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
                <label>Session timeout (idle)</label>
                <select 
                  value={sessionTimeoutMinutes} 
                  onChange={(e) => setSessionTimeoutMinutes(parseInt(e.target.value, 10))}
                >
                  <option value={5}>5 minutes</option>
                  <option value={15}>15 minutes</option>
                  <option value={30}>30 minutes</option>
                  <option value={60}>60 minutes</option>
                  <option value={120}>2 hours</option>
                </select>
              </div>
              {isAdmin && (
                <div className="setting-item">
                  <button
                    type="button"
                    onClick={() => {
                      setShowAdmin(true)
                      setIsSettingsOpen(false)
                    }}
                    style={{ padding: '0.5rem 1rem', cursor: 'pointer' }}
                  >
                    Admin panel
                  </button>
                </div>
              )}
              <div className="setting-item">
                <label>Notifications</label>
                <input type="checkbox" defaultChecked />
              </div>
              <div className="setting-item">
                <label>Auto-save</label>
                <input type="checkbox" defaultChecked />
              </div>
              {user && (
                <div className="setting-item setting-account">
                  <label>Account</label>
                  <p className="account-email">{user.email}</p>
                  <button onClick={logout} className="btn-logout">
                    Sign out
                  </button>
                </div>
              )}
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
            className={`nav-button ${view === 'routines' ? 'active' : ''}`}
            onClick={() => setView('routines')}
          >
            Routines
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
          <div className={`error-toast${restTimer.remaining !== null ? ' above-rest-bar' : ''}`} role="alert">
            <p>{error}</p>
            <button onClick={() => setError(null)} aria-label="Dismiss error">×</button>
          </div>
        )}

        {view === 'routines' && (
          <div className="routines-view">
            <div className="routines-section">
              <h2>Routine Templates</h2>
              <p className="section-desc">Create a routine from a template. Workouts and exercises are created for you.</p>
              {loading ? (
                <div className="loading-state"><p>Loading templates...</p></div>
              ) : routineTemplates.length === 0 ? (
                <p className="empty-state">No templates available.</p>
              ) : (
                <div className="template-cards">
                  {routineTemplates.map(tpl => (
                    <div key={tpl.id} className="template-card">
                      <h3>{tpl.name}</h3>
                      <p className="template-desc">{tpl.description}</p>
                      <p className="template-meta">{tpl.workout_count} workout{tpl.workout_count !== 1 ? 's' : ''}</p>
                      <button
                        className="btn-primary"
                        onClick={() => createRoutineFromTemplate(tpl.id)}
                        disabled={loading}
                      >
                        Create Routine
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div className="routines-section">
              <h2>Your Routines</h2>
              {loading ? (
                <div className="loading-state"><p>Loading routines...</p></div>
              ) : routines.length === 0 ? (
                <p className="empty-state">No routines yet. Create one from a template above.</p>
              ) : (
                <div className="routine-cards">
                  {routines.map(routine => (
                    <div key={routine.id} className="routine-card">
                      <div className="routine-header">
                        <h3>
                          {routine.name}
                          {routine.id === currentRoutineId && <span className="current-badge">Current</span>}
                        </h3>
                        <button
                          className="btn-delete"
                          onClick={() => deleteRoutine(routine.id)}
                          disabled={loading}
                        >
                          ×
                        </button>
                      </div>
                      {routine.description && (
                        <p className="routine-desc">{routine.description}</p>
                      )}
                      {routine.id !== currentRoutineId && (
                        <button className="btn-primary btn-sm use-routine" onClick={() => switchRoutine(routine.id)}>
                          Use this routine
                        </button>
                      )}
                      <div className="routine-workouts">
                        {routine.workouts?.sort((a, b) => a.slot_order - b.slot_order).map((rw, idx) => {
                          const workout = rw.workout ?? workouts.find(w => w.id === rw.workout_id)
                          return (
                            <div key={rw.id} className="routine-workout-row">
                              <span className="slot-label">Day {idx + 1}:</span>
                              <span className="workout-name">{workout?.name ?? 'Workout'}</span>
                              <button
                                className="btn-primary btn-sm"
                                onClick={() => workout && startWorkout(workout)}
                                disabled={loading || !workout}
                              >
                                {activeSession?.workout?.id === workout?.id ? 'Continue' : 'Start'}
                              </button>
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {view === 'workouts' && (
          <div className="workouts-view">
            <div className="left-panel">
              <div className="workouts-section">
                <div className="routine-switch">
                  <h2>{currentRoutine?.name ?? 'Workouts'}</h2>
                  {routines.length > 1 && (
                    <select
                      aria-label="Switch routine"
                      value={currentRoutineId ?? ''}
                      onChange={e => switchRoutine(e.target.value)}
                    >
                      {!currentRoutine && <option value="" disabled>Choose a routine</option>}
                      {routines.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
                    </select>
                  )}
                </div>
                {loading ? (
                  <div className="loading-state">
                    <p>Loading workouts...</p>
                  </div>
                ) : routines.length === 0 ? (
                  <p className="empty-state">No routine yet: add a workout to start one, or pick a template on the Routines tab.</p>
                ) : routineWorkouts.length === 0 ? (
                  <p className="empty-state">No workouts in this routine yet.</p>
                ) : (
                  <div className="workout-cards">
                    {routineWorkouts.map(workout => (
                      <div
                        key={workout.id}
                        className={`workout-card selectable${currentWorkout?.id === workout.id ? ' selected' : ''}`}
                        onClick={e => { if (!(e.target as HTMLElement).closest('button')) selectWorkout(workout) }}
                      >
                        <div className="workout-header">
                          <h3>
                            <button type="button" className="workout-select" aria-pressed={currentWorkout?.id === workout.id} onClick={() => selectWorkout(workout)}>
                              {workout.name}
                            </button>
                          </h3>
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
                {addingWorkout ? (
                  <form className="input-group add-workout-form" onSubmit={e => { e.preventDefault(); void createWorkout() }}>
                    <input
                      type="text"
                      aria-label="Workout name"
                      placeholder="Workout name..."
                      autoFocus
                      value={newWorkoutName}
                      onChange={(e) => setNewWorkoutName(e.target.value)}
                      onKeyDown={(e) => e.key === 'Escape' && cancelAddWorkout()}
                      disabled={loading}
                    />
                    <button type="submit" className="btn-primary" disabled={loading || !newWorkoutName.trim()}>
                      {loading ? 'Adding...' : 'Add'}
                    </button>
                    <button type="button" className="btn-secondary" onClick={cancelAddWorkout}>Cancel</button>
                  </form>
                ) : (
                  <button type="button" className="add-exercise-button" onClick={() => setAddingWorkout(true)} disabled={loading}>
                    + Add workout
                  </button>
                )}
              </div>
            </div>

            <div className="right-panel" ref={editorPanel}>
              {currentWorkout ? (
                <div className="current-workout">
                  <h2>Current Workout: {currentWorkout.name}</h2>
                  <div className="exercise-cards">
                    {currentWorkout.exercises?.map((exercise, index, all) => (
                      <PlannedExerciseCard
                        key={exercise.id}
                        exercise={exercise}
                        weightUnit={weightUnit}
                        convertWeight={convertWeight}
                        canMoveUp={index > 0 && !moving}
                        canMoveDown={index < all.length - 1 && !moving}
                        disabled={loading}
                        onSave={patch => savePlannedExercise(exercise, patch)}
                        onMove={delta => movePlannedExercise(exercise, delta)}
                        onRemove={() => deleteExercise(exercise.id)}
                      />
                    )) || <p>No exercises yet</p>}
                  </div>
                  <button type="button" className="add-exercise-button" onClick={() => setAddingToWorkout(true)} disabled={loading}>
                    + Add exercise
                  </button>
                  {addingToWorkout && (
                    <ExercisePicker
                      api={apiService}
                      title="Add exercise"
                      onClose={() => setAddingToWorkout(false)}
                      onPick={(_, name) => addPlannedExercise(name)}
                    />
                  )}
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
          <div className={`session-view${restTimer.remaining !== null ? ' has-rest-bar' : ''}`}>
            <SessionHeader
              workoutName={activeSession.workout?.name ?? activeSession.workout_name}
              startedAt={activeSession.started_at}
              exercises={activeSession.exercises}
              weightUnit={weightUnit}
              action={
                <button onClick={() => setFinishing(true)} className="btn-primary">
                  Finish
                </button>
              }
            />

            {activeSession.exercises?.length > 0 ? (
              <div className="session-exercises">
              {activeSession.exercises.map((sessionExercise, index, all) => (
                <section key={sessionExercise.id} id={`se-${sessionExercise.id}`} className="session-exercise">
                  <div className="session-exercise-head">
                    <h3>{sessionExercise.exercise?.name ?? sessionExercise.name}</h3>
                    <ExerciseMenu
                      name={sessionExercise.name}
                      canMoveUp={index > 0}
                      canMoveDown={index < all.length - 1}
                      onMoveUp={() => moveSessionExercise(sessionExercise, -1)}
                      onMoveDown={() => moveSessionExercise(sessionExercise, 1)}
                      onReplace={() => setPicker({ mode: 'replace', id: sessionExercise.id })}
                      onRest={() => setRestFor(sessionExercise.id)}
                      restLabel={`Rest timer: ${formatRest(sessionExercise.rest_seconds).toLowerCase()}…`}
                      onRemove={() => removeSessionExercise(sessionExercise)}
                    />
                  </div>
                  <SetTable
                    sessionExercise={sessionExercise}
                    api={apiService}
                    weightUnit={weightUnit}
                    onError={setError}
                    onSetsUpdate={updateSessionSets}
                    onSetDone={() => restTimer.start(sessionExercise.rest_seconds, sessionExercise.exercise?.name ?? sessionExercise.name)}
                  />
                </section>
              ))}
              </div>
            ) : (
              <p className="empty-state">No exercises in this session</p>
            )}

            <button type="button" className="add-exercise-button" onClick={() => setPicker({ mode: 'add' })}>
              + Add exercise
            </button>

            {finishing && (
              <FinishDialog
                api={apiService}
                sessionId={activeSession.id}
                weightUnit={weightUnit}
                busy={loading}
                error={finishError}
                onClose={() => { setFinishing(false); setFinishError(null) }}
                onFinish={finishSession}
                onDiscard={discardSession}
              />
            )}

            {restFor && (() => {
              const se = activeSession.exercises.find(x => x.id === restFor)
              return se ? (
                <RestSettingDialog
                  name={se.exercise?.name ?? se.name}
                  seconds={se.rest_seconds}
                  onPick={n => setRestSeconds(se, n)}
                  onClose={() => setRestFor(null)}
                />
              ) : null
            })()}

            <RestTimerBar timer={restTimer} />

            {picker && (
              <ExercisePicker
                api={apiService}
                title={picker.mode === 'add' ? 'Add exercise' : 'Replace exercise'}
                onClose={() => setPicker(null)}
                onPick={target => (picker.mode === 'add' ? addSessionExercise(target) : replaceSessionExercise(picker.id, target))}
              />
            )}
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
              ) : progressError ? (
                <p className="error-message">{progressError}</p>
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
              <li><button onClick={() => setView('routines')}>Routines</button></li>
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