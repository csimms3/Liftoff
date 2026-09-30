import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { RestTimerBar } from './RestTimerBar'
import type { RestTimer } from '../useRestTimer'

const timer = (over: Partial<RestTimer> = {}): RestTimer => ({
  remaining: 75, total: 90, label: 'Bench', over: false, start: vi.fn(), adjust: vi.fn(), dismiss: vi.fn(), ...over,
})

describe('RestTimerBar', () => {
  it('renders nothing without a timer', () => {
    const { container } = render(<RestTimerBar timer={timer({ remaining: null })} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('shows m:ss and the exercise, with −15, +15 and Skip', () => {
    const t = timer()
    render(<RestTimerBar timer={t} />)
    expect(screen.getByText('1:15')).toBeInTheDocument()
    expect(screen.getByText('Bench')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '15 seconds less' }))
    fireEvent.click(screen.getByRole('button', { name: '15 seconds more' }))
    fireEvent.click(screen.getByRole('button', { name: 'Skip' }))
    expect(t.adjust).toHaveBeenNthCalledWith(1, -15)
    expect(t.adjust).toHaveBeenNthCalledWith(2, 15)
    expect(t.dismiss).toHaveBeenCalled()
  })

  it('when over, says so and offers Dismiss only', () => {
    render(<RestTimerBar timer={timer({ remaining: 0, over: true })} />)
    expect(screen.getByText('Rest over')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '15 seconds more' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Dismiss' })).toBeInTheDocument()
  })
})
