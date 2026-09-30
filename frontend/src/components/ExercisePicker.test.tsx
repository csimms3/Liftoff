import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { ExercisePicker } from './ExercisePicker'

const api = () => ({
  getMovements: vi.fn().mockResolvedValue([
    { id: 'm1', name: 'Bench Press', category: 'chest', last_used: '2026-09-01' },
    { id: 'm2', name: 'Squat', category: 'legs', last_used: null },
  ]),
  getExerciseTemplates: vi.fn().mockResolvedValue([
    { name: 'Bench Press', category: 'chest', default_sets: 3, default_reps: 8, default_weight: 0 },
    { name: 'Plank', category: 'core', default_sets: 3, default_reps: 1, default_weight: 0 },
  ]),
})

describe('ExercisePicker', () => {
  it('lists your exercises, then library ones you have not used', async () => {
    render(<ExercisePicker api={api()} title="Add exercise" onPick={vi.fn()} onClose={vi.fn()} />)
    await screen.findByText('Squat')
    const names = screen.getAllByRole('button').map((b) => b.textContent ?? '').filter((t) => /Bench|Squat|Plank/.test(t))
    expect(names.map((t) => t.replace(/chest|legs|core/, ''))).toEqual(['Bench Press', 'Squat', 'Plank'])
  })

  it('picks your movement by id, a library one by name', async () => {
    const onPick = vi.fn()
    render(<ExercisePicker api={api()} title="Add exercise" onPick={onPick} onClose={vi.fn()} />)
    fireEvent.click(await screen.findByText('Squat'))
    expect(onPick).toHaveBeenCalledWith({ movement_id: 'm2' })
    fireEvent.click(screen.getByText('Plank'))
    expect(onPick).toHaveBeenCalledWith({ name: 'Plank' })
  })

  it('offers Create for a new name, but not for one that exists (any case)', async () => {
    const onPick = vi.fn()
    render(<ExercisePicker api={api()} title="Add exercise" onPick={onPick} onClose={vi.fn()} />)
    await screen.findByText('Squat')
    const search = screen.getByPlaceholderText(/search or type/i)
    fireEvent.change(search, { target: { value: ' Face Pull ' } })
    fireEvent.click(screen.getByText('Create “Face Pull”'))
    expect(onPick).toHaveBeenCalledWith({ name: 'Face Pull' })
    fireEvent.change(search, { target: { value: 'squat' } })
    expect(screen.queryByText(/Create/)).toBeNull()
  })

  it('still allows typing a name when loading fails, and closes on Escape', async () => {
    const onClose = vi.fn()
    const failing = { getMovements: vi.fn().mockRejectedValue(new Error('x')), getExerciseTemplates: vi.fn().mockRejectedValue(new Error('x')) }
    render(<ExercisePicker api={failing} title="Add exercise" onPick={vi.fn()} onClose={onClose} />)
    await waitFor(() => expect(screen.getByText(/couldn't load/i)).toBeInTheDocument())
    fireEvent.change(screen.getByPlaceholderText(/search or type/i), { target: { value: 'Row' } })
    expect(screen.getByText('Create “Row”')).toBeInTheDocument()
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })
})

describe('ExercisePicker keyboard', () => {
  it('Enter picks the exact or only match, otherwise creates the typed name', async () => {
    const onPick = vi.fn()
    render(<ExercisePicker api={api()} title="Add exercise" onPick={onPick} onClose={vi.fn()} />)
    await screen.findByText('Squat')
    const search = screen.getByPlaceholderText(/search or type/i)
    fireEvent.change(search, { target: { value: 'squ' } })
    fireEvent.keyDown(search, { key: 'Enter' })
    expect(onPick).toHaveBeenLastCalledWith({ movement_id: 'm2' }) // only match
    fireEvent.change(search, { target: { value: 'face pull' } })
    fireEvent.keyDown(search, { key: 'Enter' })
    expect(onPick).toHaveBeenLastCalledWith({ name: 'face pull' }) // no match: create
  })
})
