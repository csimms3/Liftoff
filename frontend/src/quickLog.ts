import type { ApiService, WorkoutSession } from './api'

type QuickLogApi = Pick<ApiService, 'createSession' | 'addExerciseToSession' | 'createSet' | 'updateSet' | 'endSession'>

/**
 * Logs one completed set for an exercise outside the normal session flow.
 *
 * During a workout the set goes into the active session (reusing the exercise's
 * row if it's already there). Starting a session would end the one in progress,
 * so a throwaway session is only used when nothing is active.
 */
export async function quickLogSet(
  api: QuickLogApi,
  activeSession: WorkoutSession | null,
  workoutId: string,
  exerciseId: string,
  reps: number,
  weight: number,
  notes?: string,
): Promise<void> {
  if (activeSession) {
    const existing = activeSession.exercises?.find((se) => se.exercise_id === exerciseId)
    const sessionExercise = existing ?? (await api.addExerciseToSession(activeSession.id, exerciseId))
    const set = await api.createSet(sessionExercise.id, reps, weight)
    await api.updateSet(set.id, reps, weight, notes)
    return
  }
  const session = await api.createSession(workoutId)
  const sessionExercise = await api.addExerciseToSession(session.id, exerciseId)
  const set = await api.createSet(sessionExercise.id, reps, weight)
  await api.updateSet(set.id, reps, weight, notes)
  await api.endSession(session.id)
}
