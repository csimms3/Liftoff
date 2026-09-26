import { describe, it, expect, vi } from 'vitest'
import { quickLogSet } from './quickLog'
import type { WorkoutSession } from './api'

function fakeApi() {
  return {
    createSession: vi.fn().mockResolvedValue({ id: 'tmp-session' }),
    addExerciseToSession: vi.fn().mockResolvedValue({ id: 'new-se' }),
    createSet: vi.fn().mockResolvedValue({ id: 'set-1' }),
    updateSet: vi.fn().mockResolvedValue(undefined),
    endSession: vi.fn().mockResolvedValue({}),
  }
}

const active = {
  id: 'active-session',
  exercises: [{ id: 'se-bench', exercise_id: 'bench', sets: [] }],
} as unknown as WorkoutSession

describe('quickLogSet', () => {
  it('logs into the active workout instead of starting (and so ending) a session', async () => {
    const api = fakeApi()
    await quickLogSet(api, active, 'w1', 'bench', 5, 100, 'easy')
    expect(api.createSession).not.toHaveBeenCalled()
    expect(api.endSession).not.toHaveBeenCalled()
    expect(api.addExerciseToSession).not.toHaveBeenCalled()
    expect(api.createSet).toHaveBeenCalledWith('se-bench', 5, 100)
    expect(api.updateSet).toHaveBeenCalledWith('set-1', 5, 100, 'easy')
  })

  it('adds the exercise to the active session when it is not in it yet', async () => {
    const api = fakeApi()
    await quickLogSet(api, active, 'w1', 'squat', 3, 140)
    expect(api.createSession).not.toHaveBeenCalled()
    expect(api.addExerciseToSession).toHaveBeenCalledWith('active-session', 'squat')
    expect(api.createSet).toHaveBeenCalledWith('new-se', 3, 140)
  })

  it('uses a throwaway session when no workout is active', async () => {
    const api = fakeApi()
    await quickLogSet(api, null, 'w1', 'bench', 5, 100)
    expect(api.createSession).toHaveBeenCalledWith('w1')
    expect(api.endSession).toHaveBeenCalledWith('tmp-session')
  })
})
