import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import App from './App'
import { AuthProvider } from './context/AuthContext'

// Mock fetch for API calls
const mockFetch = vi.fn()
beforeEach(() => {
  vi.stubGlobal('fetch', mockFetch)
  localStorage.clear()
  // Pre-populate auth so user is logged in
  localStorage.setItem('liftoff-auth', JSON.stringify({
    token: 'test-token',
    user: { id: '1', email: 'test@test.com' },
    expiresAt: new Date(Date.now() + 86400000).toISOString(),
  }))
})
afterEach(() => {
  vi.unstubAllGlobals()
})

function renderWithAuth(ui: React.ReactElement) {
  return render(
    <AuthProvider>
      {ui}
    </AuthProvider>
  )
}

describe('App', () => {
  beforeEach(() => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/workouts')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve([]) })
      }
      return Promise.resolve({ ok: false })
    })
  })

  test('renders app title', async () => {
    renderWithAuth(<App />)
    await waitFor(() => {
      const headings = screen.getAllByRole('heading', { name: /liftoff/i })
      expect(headings.length).toBeGreaterThan(0)
    })
  })

  test('shows create workout form', async () => {
    renderWithAuth(<App />)
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Workout name...')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create/i })).toBeInTheDocument()
    })
  })

  test('shows loading state initially', async () => {
    renderWithAuth(<App />)
    await waitFor(() => {
      expect(screen.getByText('Loading workouts...')).toBeInTheDocument()
    })
  })
})

describe('App — workout in progress', () => {
  const workout = { id: 'w1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
  const active = { id: 's1', workout_id: 'w1', workout, started_at: '', is_active: true, exercises: [] }

  beforeEach(() => {
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      const json = (body: unknown) => Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body) })
      if (url.includes('/sessions/active')) return json(active)
      if (url.includes('/sessions') && init?.method === 'POST') return json(active)
      if (url.endsWith('/workouts')) return json([workout])
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
    })
  })

  test('"Continue Session" shows the session without starting (and so ending) one', async () => {
    renderWithAuth(<App />)
    const button = await screen.findByRole('button', { name: 'Continue Session' })
    fireEvent.click(button)
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Continue Session' })).toBeNull())
    const started = mockFetch.mock.calls.filter(([url, init]) => String(url).endsWith('/sessions') && init?.method === 'POST')
    expect(started).toHaveLength(0)
  })
})

// The set rows keep their own state; App's copy of the session must follow it,
// or switching views and back shows (and re-saves) stale values.
describe('App — set edits survive switching views', () => {
  test('edited and ticked set is still edited and ticked after Workouts -> Active Session', async () => {
    const workout = { id: 'w1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
    const server = { id: 'a', weight: 100, reps: 8, completed: false }
    const session = () => ({
      id: 's1', workout_id: 'w1', workout, started_at: '', is_active: true,
      exercises: [{ id: 'se1', exercise_id: 'e1', exercise: { id: 'e1', name: 'Bench' }, sets: [{ ...server }], previous: [] }],
    })
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      const json = (body: unknown) => Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body) })
      if (url.includes('/exercise-sets/a') && init?.method === 'PATCH') {
        Object.assign(server, JSON.parse(String(init.body)))
        return json({ ...server })
      }
      if (url.includes('/sessions/active')) return json(session())
      if (url.endsWith('/workouts')) return json([workout])
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
    })
    renderWithAuth(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Continue Session' }))

    const weight = await screen.findByLabelText('Set 1 weight')
    fireEvent.change(weight, { target: { value: '110' } })
    fireEvent.blur(weight)
    await waitFor(() => expect(server.weight).toBe(110))
    fireEvent.click(screen.getByRole('button', { name: 'Set 1 done' }))
    await waitFor(() => expect(server.completed).toBe(true))

    fireEvent.click(screen.getAllByRole('button', { name: 'Workouts' })[0]) // nav, not the footer link
    fireEvent.click(screen.getByRole('button', { name: 'Active Session' }))
    expect(await screen.findByLabelText('Set 1 weight')).toHaveValue(110)
    expect(screen.getByRole('button', { name: 'Set 1 done' })).toHaveAttribute('aria-pressed', 'true')
  })

  test('a save still in flight when leaving the session view is kept', async () => {
    const workout = { id: 'w1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
    const server = { id: 'a', weight: 100, reps: 8, completed: false }
    const session = () => ({
      id: 's1', workout_id: 'w1', workout, started_at: '', is_active: true,
      exercises: [{ id: 'se1', exercise_id: 'e1', exercise: { id: 'e1', name: 'Bench' }, sets: [{ ...server }], previous: [] }],
    })
    let sessionLoads = 0
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      const json = (body: unknown) => Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body) })
      if (url.includes('/exercise-sets/a') && init?.method === 'PATCH') {
        Object.assign(server, JSON.parse(String(init.body)))
        const reply = { ...server }
        return new Promise(resolve => setTimeout(() => resolve({ ok: true, status: 200, json: () => Promise.resolve(reply) }), 50))
      }
      if (url.includes('/sessions/active')) { sessionLoads++; return json(session()) }
      if (url.endsWith('/workouts')) return json([workout])
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
    })
    renderWithAuth(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Continue Session' }))
    const loadsBefore = sessionLoads

    const weight = await screen.findByLabelText('Set 1 weight')
    fireEvent.change(weight, { target: { value: '110' } })
    fireEvent.blur(weight)
    fireEvent.click(screen.getAllByRole('button', { name: 'Workouts' })[0]) // leave before the reply
    await new Promise(r => setTimeout(r, 150))
    fireEvent.click(screen.getByRole('button', { name: 'Active Session' }))

    expect(await screen.findByLabelText('Set 1 weight')).toHaveValue(110)
    expect(sessionLoads).toBe(loadsBefore) // from App's copy, not a reload
  })
})

describe('App — editing exercises mid-session', () => {
  const workout = { id: 'w1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
  const sets = (id: string, completed = false) => [{ id: `${id}-1`, weight: 100, reps: 8, completed }]
  const mk = (id: string, name: string, completed = false) => ({
    id, exercise_id: '', movement_id: `m-${id}`, name, exercise: { id: '', name }, sets: sets(id, completed), previous: [],
  })
  let server: ReturnType<typeof mk>[]
  const calls: string[] = []

  beforeEach(() => {
    calls.length = 0
    server = [mk('a', 'Bench', true), mk('b', 'Row')]
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      const json = (body: unknown, status = 200) => Promise.resolve({ ok: true, status, json: () => Promise.resolve(body) })
      const method = init?.method ?? 'GET'
      if (url.includes('/sessions/active')) return json({ id: 's1', workout_id: 'w1', workout, started_at: '', is_active: true, exercises: server })
      if (url.endsWith('/workouts')) return json([workout])
      if (url.endsWith('/movements') && method === 'GET') return json([{ id: 'm-x', name: 'Squat', category: 'legs', last_used: null }])
      if (url.endsWith('/exercise-templates')) return json([])
      if (url.endsWith('/sessions/s1/movements') && method === 'POST') {
        calls.push('add ' + init?.body)
        const added = mk('c', 'Squat')
        server.push(added)
        return json(added, 201)
      }
      const del = url.match(/\/sessions\/exercises\/(\w+)$/)
      if (del && method === 'DELETE') { calls.push('remove ' + del[1]); return json(undefined, 204) }
      if (del && method === 'PATCH') { calls.push('move ' + del[1] + ' ' + init?.body); return json(undefined, 204) }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
    })
  })

  const open = async () => {
    renderWithAuth(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Continue Session' }))
    await screen.findByRole('button', { name: '+ Add exercise' })
  }

  test('add exercise from the picker appends it', async () => {
    await open()
    fireEvent.click(screen.getByRole('button', { name: '+ Add exercise' }))
    fireEvent.click(await screen.findByText('Squat'))
    await screen.findByRole('heading', { name: 'Squat' })
    expect(calls).toEqual(['add {"movement_id":"m-x"}'])
  })

  test('removing an exercise with logged sets asks first; without, it does not', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Bench options' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Remove' }))
    expect(confirm).toHaveBeenCalledOnce()
    expect(calls).toEqual([])

    fireEvent.click(screen.getByRole('button', { name: 'Row options' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Remove' }))
    await waitFor(() => expect(screen.queryByRole('heading', { name: 'Row' })).toBeNull())
    expect(confirm).toHaveBeenCalledOnce() // no prompt for Row (nothing logged)
    expect(calls).toEqual(['remove b'])
    confirm.mockRestore()
  })

  test('move down reorders and tells the server the new position', async () => {
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Bench options' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Move down' }))
    await waitFor(() => expect(calls).toEqual(['move a {"position":1}']))
    const headings = screen.getAllByRole('heading', { level: 3 }).map(h => h.textContent)
    expect(headings.indexOf('Row')).toBeLessThan(headings.indexOf('Bench'))
  })
})

describe('App — finishing a workout', () => {
  test('Finish opens the summary and finishing ends the session', async () => {
    const workout = { id: 'w1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
    let active = true
    const calls: string[] = []
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      const json = (body: unknown) => Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body) })
      const method = init?.method ?? 'GET'
      if (url.includes('/sessions/active')) return active ? json({ id: 's1', workout_id: 'w1', workout, started_at: '', is_active: true, exercises: [] }) : Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
      if (url.endsWith('/sessions/s1/summary')) return json({ session_id: 's1', workout_id: 'w1', workout_name: 'Push', duration_seconds: 600, sets_done: 2, sets_total: 4, volume: 500, can_update_workout: true, changes: { added: ['Lunge'], removed: [], set_counts: [], reordered: false, has_changes: true } })
      if (url.endsWith('/sessions/s1/finish') && method === 'POST') { calls.push('finish ' + init?.body); active = false; return json({}) }
      if (url.endsWith('/workouts')) return json([workout])
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
    })
    renderWithAuth(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Continue Session' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Finish' }))
    await screen.findByText('Add Lunge')
    fireEvent.click(screen.getByRole('button', { name: 'Finish workout' }))
    await waitFor(() => expect(calls).toEqual(['finish {"update_workout":true}']))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Finish workout' })).toBeNull())
  })
})

describe('App — Finish failures', () => {
  const workout = { id: 'w1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
  const setup = (finishStatus: number) => {
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      const json = (body: unknown) => Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body) })
      if (url.includes('/sessions/active')) return json({ id: 's1', workout_id: 'w1', workout, started_at: '', is_active: true, exercises: [] })
      if (url.endsWith('/sessions/s1/summary')) return json({ session_id: 's1', workout_id: 'w1', workout_name: 'Push', duration_seconds: 60, sets_done: 1, sets_total: 2, volume: 10, can_update_workout: true, changes: { added: [], removed: [], set_counts: [], reordered: false, has_changes: false } })
      if (url.endsWith('/sessions/s1/finish') && init?.method === 'POST') return Promise.resolve({ ok: false, status: finishStatus, json: () => Promise.resolve({}) })
      if (url.endsWith('/workouts')) return json([workout])
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
    })
  }
  const openFinish = async () => {
    renderWithAuth(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Continue Session' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Finish' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Finish workout' }))
  }

  test('a server error stays in the dialog so you can retry', async () => {
    setup(500)
    await openFinish()
    expect(await screen.findByRole('alert')).toHaveTextContent("Couldn't finish the workout")
    expect(screen.getByRole('dialog', { name: 'Finish workout' })).toBeInTheDocument()
  })

  test('a session that already ended closes the dialog and says so', async () => {
    setup(404)
    await openFinish()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Finish workout' })).toBeNull())
    expect(await screen.findByText('That workout has already ended')).toBeInTheDocument()
  })
})

