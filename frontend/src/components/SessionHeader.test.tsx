import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { SessionHeader } from './SessionHeader'
import { formatElapsed, sessionStats } from '../sessionStats'
import type { SessionExercise, ExerciseSet } from '../api'

const set = (weight: number, reps: number, completed: boolean): ExerciseSet => ({ id: `${weight}-${reps}-${completed}`, weight, reps, completed })
const ex = (sets: ExerciseSet[]): SessionExercise =>
  ({ id: 'e', exercise_id: '', movement_id: 'm', name: 'Bench', rest_seconds: 90, exercise: { id: '', name: 'Bench' }, sets, previous: [] }) as unknown as SessionExercise

describe('formatElapsed', () => {
  it('formats m:ss and h:mm:ss, never negative', () => {
    expect(formatElapsed(0)).toBe('0:00')
    expect(formatElapsed(65_000)).toBe('1:05')
    expect(formatElapsed(59 * 60_000 + 59_000)).toBe('59:59')
    expect(formatElapsed(3_600_000)).toBe('1:00:00')
    expect(formatElapsed(3 * 3600_000 + 7 * 60_000 + 9_000)).toBe('3:07:09')
    expect(formatElapsed(-5000)).toBe('0:00')
  })
})

describe('sessionStats', () => {
  it('counts done and total sets and volume of done sets only', () => {
    const s = sessionStats([ex([set(100, 5, true), set(100, 5, false)]), ex([set(50, 10, true)])])
    expect(s).toEqual({ done: 2, total: 3, volume: 1000 })
  })
  it('handles no exercises', () => {
    expect(sessionStats([])).toEqual({ done: 0, total: 0, volume: 0 })
  })
})

describe('SessionHeader', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-30T12:10:00Z'))
  })
  afterEach(() => vi.useRealTimers())

  const render1 = (unit: 'lbs' | 'kg' = 'lbs') =>
    render(<SessionHeader workoutName="Push" startedAt="2026-09-30T12:00:00Z" exercises={[ex([set(200, 5, true), set(100, 5, false)])]} weightUnit={unit} action={<button>Finish</button>} />)

  it('shows the workout, live elapsed time, sets, volume and the action', () => {
    render1()
    expect(screen.getByRole('heading', { name: 'Push' })).toBeInTheDocument()
    expect(screen.getByLabelText('Elapsed time')).toHaveTextContent('10:00')
    expect(screen.getByText('1/2')).toBeInTheDocument()
    expect(screen.getByText('1,000 lbs')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Finish' })).toBeInTheDocument()
    act(() => { vi.advanceTimersByTime(5_000) })
    expect(screen.getByLabelText('Elapsed time')).toHaveTextContent('10:05')
  })

  it('shows volume in kg', () => {
    render1('kg')
    expect(screen.getByText('454 kg')).toBeInTheDocument()
  })

  it('catches up after a suspended tab', () => {
    render1()
    act(() => { vi.setSystemTime(Date.now() + 120_000); document.dispatchEvent(new Event('visibilitychange')) })
    expect(screen.getByLabelText('Elapsed time')).toHaveTextContent('12:00')
  })
})
