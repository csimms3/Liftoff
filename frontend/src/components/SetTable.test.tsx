import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SetTable } from './SetTable'
import type { ExerciseSet, SessionExercise } from '../api'

const set = (id: string, weight: number, reps: number, completed = false): ExerciseSet => ({ id, weight, reps, completed })

function exercise(sets: ExerciseSet[], previous: ExerciseSet[] = []): SessionExercise {
  return {
    id: 'se1',
    exercise_id: 'bench',
    movement_id: 'm-bench',
    name: 'Bench',
    exercise: { id: 'bench', name: 'Bench', sets: 3, reps: 8, weight: 100, workout_id: 'w1' } as SessionExercise['exercise'],
    sets,
    previous,
  }
}

function fakeApi() {
  return {
    // Like the server: fields left out (undefined) keep their stored value.
    patchSet: vi.fn(async (id: string, patch: Partial<ExerciseSet>) => ({
      ...set(id, 100, 8),
      ...Object.fromEntries(Object.entries(patch).filter(([, v]) => v !== undefined)),
    })),
    createSet: vi.fn(async () => set('new', 0, 0)),
    deleteSet: vi.fn(async () => undefined),
  }
}

function renderTable(se: SessionExercise, api = fakeApi(), weightUnit: 'lbs' | 'kg' = 'lbs') {
  const onError = vi.fn()
  const onSetsUpdate = vi.fn()
  const view = render(
    <SetTable sessionExercise={se} api={api} weightUnit={weightUnit} onError={onError} onSetsUpdate={onSetsUpdate} />,
  )
  const rerender = (next: SessionExercise) =>
    view.rerender(<SetTable sessionExercise={next} api={api} weightUnit={weightUnit} onError={onError} onSetsUpdate={onSetsUpdate} />)
  return { api, onError, onSetsUpdate, rerender }
}

describe('SetTable', () => {
  afterEach(() => vi.restoreAllMocks())

  it('shows one row per set with previous values and editable weight/reps', () => {
    renderTable(exercise([set('a', 100, 8), set('b', 105, 6)], [set('p1', 95, 8)]))
    expect(screen.getByLabelText('Set 1 weight')).toHaveValue(100)
    expect(screen.getByLabelText('Set 2 reps')).toHaveValue(6)
    expect(screen.getByText('95 × 8')).toBeInTheDocument()
    expect(screen.getAllByText('—')).toHaveLength(1) // set 2 has no previous
  })

  it('saves an edit when the field loses focus, only if it changed', async () => {
    const { api } = renderTable(exercise([set('a', 100, 8)]))
    const reps = screen.getByLabelText('Set 1 reps')
    fireEvent.blur(reps)
    expect(api.patchSet).not.toHaveBeenCalled()

    fireEvent.change(reps, { target: { value: '10' } })
    fireEvent.blur(reps)
    await waitFor(() => expect(api.patchSet).toHaveBeenCalledWith('a', { reps: 10 }))
    expect(api.patchSet.mock.calls[0][1]).not.toHaveProperty('weight', 100) // untouched field not re-sent
  })

  it('puts the saved value back for an invalid entry', () => {
    const { api } = renderTable(exercise([set('a', 100, 8)]))
    const reps = screen.getByLabelText('Set 1 reps')
    fireEvent.change(reps, { target: { value: '' } })
    fireEvent.blur(reps)
    expect(reps).toHaveValue(8)
    expect(api.patchSet).not.toHaveBeenCalled()
  })

  it('shows and edits weight in kg but saves lbs', async () => {
    const { api } = renderTable(exercise([set('a', 100, 8)]), fakeApi(), 'kg')
    const weight = screen.getByLabelText('Set 1 weight')
    expect(weight).toHaveValue(45.4)
    fireEvent.change(weight, { target: { value: '50' } })
    fireEvent.blur(weight)
    await waitFor(() => expect(api.patchSet).toHaveBeenCalledWith('a', { weight: 110.23 }))
  })

  it('✓ toggles completion both ways', async () => {
    const user = userEvent.setup()
    const { api } = renderTable(exercise([set('a', 100, 8)]))
    const check = screen.getByRole('button', { name: 'Set 1 done' })
    await user.click(check)
    expect(check).toHaveAttribute('aria-pressed', 'true')
    await waitFor(() => expect(api.patchSet).toHaveBeenLastCalledWith('a', { completed: true }))
    await user.click(check)
    expect(check).toHaveAttribute('aria-pressed', 'false')
    await waitFor(() => expect(api.patchSet).toHaveBeenLastCalledWith('a', { completed: false }))
  })

  it('keeps ✓ on when an earlier save for the set replies after the tick', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    let releaseFirst: () => void = () => {}
    api.patchSet.mockImplementationOnce(
      (id: string) => new Promise((resolve) => { releaseFirst = () => resolve({ ...set(id, 100, 12), completed: false }) }),
    )
    renderTable(exercise([set('a', 100, 8)]), api)
    const reps = screen.getByLabelText('Set 1 reps')
    await user.clear(reps)
    await user.type(reps, '12')
    await user.click(screen.getByRole('button', { name: 'Set 1 done' })) // blur save, then tick
    releaseFirst()
    await waitFor(() => expect(api.patchSet).toHaveBeenCalledTimes(2))
    expect(api.patchSet.mock.calls[1][1]).toMatchObject({ completed: true })
    expect(screen.getByRole('button', { name: 'Set 1 done' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('+ adds a set copying the last one', async () => {
    const user = userEvent.setup()
    const { api } = renderTable(exercise([set('a', 100, 8), set('b', 105, 6)]))
    await user.click(screen.getByRole('button', { name: 'Add Bench set' }))
    expect(api.createSet).toHaveBeenCalledWith('se1', 6, 105)
    expect(await screen.findByLabelText('Set 3 weight')).toHaveValue(105)
  })

  it('− removes the last set, asking first only if it was logged, and keeps one', async () => {
    const user = userEvent.setup()
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const { api } = renderTable(exercise([set('a', 100, 8), set('b', 100, 8, true)]))
    const minus = screen.getByRole('button', { name: 'Remove last Bench set' })

    await user.click(minus) // last set is logged: asks, user says no
    expect(confirm).toHaveBeenCalled()
    expect(api.deleteSet).not.toHaveBeenCalled()

    confirm.mockReturnValue(true)
    await user.click(minus)
    expect(api.deleteSet).toHaveBeenCalledWith('b')
    await waitFor(() => expect(screen.queryByLabelText('Set 2 weight')).toBeNull())
    expect(minus).toBeDisabled() // one set left
  })

  it('reports failures and restores the row', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    api.patchSet.mockRejectedValueOnce(new Error('HTTP error! status: 500'))
    const { onError } = renderTable(exercise([set('a', 100, 8)]), api)
    const check = screen.getByRole('button', { name: 'Set 1 done' })
    await user.click(check)
    await waitFor(() => expect(onError).toHaveBeenCalledWith('Failed to update set'))
    expect(check).toHaveAttribute('aria-pressed', 'false')
  })

  it('in kg, saving or ticking never re-sends an untouched weight (no rounding drift)', async () => {
    const user = userEvent.setup()
    const { api } = renderTable(exercise([set('a', 100, 8)]), fakeApi(), 'kg')
    const reps = screen.getByLabelText('Set 1 reps')
    fireEvent.change(reps, { target: { value: '9' } })
    fireEvent.blur(reps)
    await waitFor(() => expect(api.patchSet).toHaveBeenCalledTimes(1))
    await user.click(screen.getByRole('button', { name: 'Set 1 done' }))
    await waitFor(() => expect(api.patchSet).toHaveBeenCalledTimes(2))
    for (const [, patch] of api.patchSet.mock.calls) {
      expect(patch.weight).toBeUndefined()
    }
  })

  it('+ right after editing the last set copies the edited values', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    api.patchSet.mockImplementation(() => new Promise(() => {})) // the blur save never finishes
    renderTable(exercise([set('a', 100, 8)]), api)
    const weight = screen.getByLabelText('Set 1 weight')
    await user.clear(weight)
    await user.type(weight, '120')
    await user.click(screen.getByRole('button', { name: 'Add Bench set' }))
    expect(api.createSet).toHaveBeenCalledWith('se1', 8, 120)
    expect(await screen.findByLabelText('Set 2 weight')).toHaveValue(120)
  })

  it('reports each confirmed change to the parent', async () => {
    const user = userEvent.setup()
    const { onSetsUpdate } = renderTable(exercise([set('a', 100, 8)]))
    const apply = (i: number, sets: ExerciseSet[]) => onSetsUpdate.mock.calls[i][1](sets)

    await user.click(screen.getByRole('button', { name: 'Set 1 done' }))
    await waitFor(() => expect(onSetsUpdate).toHaveBeenCalledTimes(1))
    expect(apply(0, [set('a', 100, 8)])).toEqual([expect.objectContaining({ id: 'a', completed: true })])

    await user.click(screen.getByRole('button', { name: 'Add Bench set' }))
    await waitFor(() => expect(onSetsUpdate).toHaveBeenCalledTimes(2))
    expect(apply(1, [set('a', 100, 8)]).map((s: ExerciseSet) => s.id)).toEqual(['a', 'new'])

    await user.click(screen.getByRole('button', { name: 'Remove last Bench set' }))
    await waitFor(() => expect(onSetsUpdate).toHaveBeenCalledTimes(3))
    expect(apply(2, [set('a', 100, 8), set('new', 100, 8)]).map((s: ExerciseSet) => s.id)).toEqual(['a'])
  })

  it('a save landing after a reload from the server shows the saved value', async () => {
    const api = fakeApi()
    let reply: () => void = () => {}
    api.patchSet.mockImplementationOnce(
      (id: string) => new Promise((resolve) => { reply = () => resolve(set(id, 120, 8)) }),
    )
    const { rerender } = renderTable(exercise([set('a', 100, 8)]), api)
    const weight = screen.getByLabelText('Set 1 weight')
    fireEvent.change(weight, { target: { value: '120' } })
    fireEvent.blur(weight)
    await waitFor(() => expect(api.patchSet).toHaveBeenCalled())
    rerender(exercise([set('a', 110, 5)])) // an older copy arrives while the save is in flight
    expect(weight).toHaveValue(110)
    reply()
    await waitFor(() => expect(weight).toHaveValue(120))
  })

  it('a reset from the server keeps unchanged rows being typed in, and a pending tick lands', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    let tickReply: () => void = () => {}
    api.patchSet.mockImplementationOnce(
      (id: string) => new Promise((resolve) => { tickReply = () => resolve({ ...set(id, 100, 8), completed: true }) }),
    )
    const { rerender } = renderTable(exercise([set('a', 100, 8), set('b', 100, 8)]), api)
    await user.click(screen.getByRole('button', { name: 'Set 2 done' }))
    await waitFor(() => expect(api.patchSet).toHaveBeenCalled())
    fireEvent.change(screen.getByLabelText('Set 2 reps'), { target: { value: '12' } })

    // A server copy where only set 1 changed arrives while the tick is saving.
    rerender(exercise([set('a', 105, 8), set('b', 100, 8)]))
    expect(screen.getByLabelText('Set 1 weight')).toHaveValue(105)
    expect(screen.getByLabelText('Set 2 reps')).toHaveValue(12) // still being typed
    expect(screen.getByRole('button', { name: 'Set 2 done' })).toHaveAttribute('aria-pressed', 'true')

    tickReply()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Set 2 done' })).toHaveAttribute('aria-pressed', 'true'))
  })
})
