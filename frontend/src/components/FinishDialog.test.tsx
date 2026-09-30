import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { FinishDialog } from './FinishDialog'
import type { SessionSummary } from '../api'

const summary = (over: Partial<SessionSummary> = {}): SessionSummary => ({
  session_id: 's1', workout_id: 'w1', workout_name: 'Push', duration_seconds: 2700,
  sets_done: 6, sets_total: 9, volume: 2000,
  changes: { added: ['Lunge'], removed: ['Dips'], set_counts: [{ name: 'Bench', from: 3, to: 4 }], reordered: true, has_changes: true },
  can_update_workout: true, ...over,
})

function setup(s: SessionSummary, unit: 'lbs' | 'kg' = 'lbs') {
  const handlers = { onFinish: vi.fn(), onDiscard: vi.fn(), onClose: vi.fn() }
  render(<FinishDialog api={{ getSessionSummary: vi.fn().mockResolvedValue(s) }} sessionId="s1" weightUnit={unit} {...handlers} />)
  return handlers
}

describe('FinishDialog', () => {
  it('shows time, sets and volume in the chosen unit', async () => {
    setup(summary(), 'kg')
    expect(await screen.findByText('45 min')).toBeInTheDocument()
    expect(screen.getByText('6 / 9')).toBeInTheDocument()
    expect(screen.getByText('907 kg')).toBeInTheDocument()
  })

  it('offers the workout update (on by default) and lists the changes', async () => {
    const h = setup(summary())
    const box = await screen.findByRole('checkbox')
    expect(box).toBeChecked()
    for (const t of ['Add Lunge', 'Remove Dips', 'Bench: 3 → 4 sets', 'New exercise order']) expect(screen.getByText(t)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Finish workout' }))
    expect(h.onFinish).toHaveBeenCalledWith(true)
  })

  it('unticking keeps the workout as is', async () => {
    const h = setup(summary())
    fireEvent.click(await screen.findByRole('checkbox'))
    fireEvent.click(screen.getByRole('button', { name: 'Finish workout' }))
    expect(h.onFinish).toHaveBeenCalledWith(false)
  })

  it('no offer when nothing changed or the workout is gone', async () => {
    setup(summary({ changes: { added: [], removed: [], set_counts: [], reordered: false, has_changes: false } }))
    await screen.findByRole('button', { name: 'Finish workout' })
    expect(screen.queryByRole('checkbox')).toBeNull()
  })

  it('no offer when the workout was deleted', async () => {
    setup(summary({ can_update_workout: false }))
    await screen.findByRole('button', { name: 'Finish workout' })
    expect(screen.queryByRole('checkbox')).toBeNull()
  })

  it('nothing logged offers Discard, Save anyway and Keep going', async () => {
    const h = setup(summary({ sets_done: 0, volume: 0 }))
    fireEvent.click(await screen.findByRole('button', { name: 'Discard workout' }))
    expect(h.onDiscard).toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Save anyway' }))
    expect(h.onFinish).toHaveBeenCalledWith(false)
    fireEvent.click(screen.getByRole('button', { name: 'Keep going' }))
    expect(h.onClose).toHaveBeenCalled()
    expect(screen.queryByRole('checkbox')).toBeNull()
  })

  it('still lets you finish when the summary fails to load', async () => {
    const h = { onFinish: vi.fn(), onDiscard: vi.fn(), onClose: vi.fn() }
    render(<FinishDialog api={{ getSessionSummary: vi.fn().mockRejectedValue(new Error('x')) }} sessionId="s1" weightUnit="lbs" {...h} />)
    await waitFor(() => expect(screen.getByText(/couldn't load/i)).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Finish anyway' }))
    expect(h.onFinish).toHaveBeenCalledWith(false)
  })
})

describe('FinishDialog errors', () => {
  it('shows a failure inside the dialog', async () => {
    render(<FinishDialog api={{ getSessionSummary: vi.fn().mockResolvedValue(summary()) }} sessionId="s1" weightUnit="lbs"
      onFinish={vi.fn()} onDiscard={vi.fn()} onClose={vi.fn()} error="Couldn't finish the workout. Try again." />)
    expect(await screen.findByRole('alert')).toHaveTextContent("Couldn't finish")
    expect(screen.getByRole('button', { name: 'Finish workout' })).toBeInTheDocument() // can retry
  })
})
