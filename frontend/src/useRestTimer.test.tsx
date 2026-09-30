import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useRestTimer } from './useRestTimer'

describe('useRestTimer', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-30T12:00:00Z'))
    localStorage.clear()
  })
  afterEach(() => vi.useRealTimers())

  it('counts down from real time, not from ticks', () => {
    const { result } = renderHook(() => useRestTimer('s1', vi.fn()))
    expect(result.current.remaining).toBeNull()
    act(() => result.current.start(90, 'Bench'))
    expect(result.current.remaining).toBe(90)
    act(() => { vi.advanceTimersByTime(30_000) })
    expect(result.current.remaining).toBe(60)
    // A suspended tab: the clock jumps 40 s without any interval firing.
    act(() => { vi.setSystemTime(Date.now() + 40_000); document.dispatchEvent(new Event('visibilitychange')) })
    expect(result.current.remaining).toBe(20)
  })

  it('alerts once when it reaches zero, shows "over", then hides', () => {
    const onOver = vi.fn()
    const { result } = renderHook(() => useRestTimer('s1', onOver))
    act(() => result.current.start(10, 'Bench'))
    act(() => { vi.advanceTimersByTime(10_500) })
    expect(result.current.over).toBe(true)
    expect(result.current.remaining).toBe(0)
    act(() => { vi.advanceTimersByTime(3000) })
    expect(onOver).toHaveBeenCalledTimes(1)
    act(() => { vi.advanceTimersByTime(6000) })
    expect(result.current.remaining).toBeNull()
  })

  it('adjusts by 15 s, never below a second left, and can be dismissed', () => {
    const { result } = renderHook(() => useRestTimer('s1', vi.fn()))
    act(() => result.current.start(90, 'Bench'))
    act(() => result.current.adjust(15))
    expect(result.current.remaining).toBe(105)
    act(() => result.current.adjust(-200))
    expect(result.current.remaining).toBe(1)
    act(() => result.current.dismiss())
    expect(result.current.remaining).toBeNull()
  })

  it('a rest of 0 (off) starts nothing', () => {
    const { result } = renderHook(() => useRestTimer('s1', vi.fn()))
    act(() => result.current.start(0, 'Bench'))
    expect(result.current.remaining).toBeNull()
  })

  it('survives a reload for the same session only', () => {
    const first = renderHook(() => useRestTimer('s1', vi.fn()))
    act(() => first.result.current.start(60, 'Bench'))
    first.unmount()
    const same = renderHook(() => useRestTimer('s1', vi.fn()))
    expect(same.result.current.remaining).toBe(60)
    same.unmount()
    const other = renderHook(() => useRestTimer('s2', vi.fn()))
    expect(other.result.current.remaining).toBeNull()
  })
})
