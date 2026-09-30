import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import App from './App'
import { AuthProvider } from './context/AuthContext'
import { formatRest } from './formatRest'

// Mock fetch for API calls
const mockFetch = vi.fn()

// The routines endpoints are served from this state unless a test changes it; everything else goes to mockFetch.
const routineOf = (id: string, name: string, workoutIds: string[] = []) => ({
  id, name, description: '', created_at: '', updated_at: '',
  workouts: workoutIds.map((w, i) => ({ id: w, routine_id: id, workout_id: w, slot_order: i + 1 })),
})
let routineState: { current: string | null; routines: ReturnType<typeof routineOf>[]; puts: string[] }
function routineFetch(url: string, init?: RequestInit) {
  const json = (body: unknown) => Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body) })
  const method = init?.method ?? 'GET'
  if (url.endsWith('/routines/current')) {
    if (method === 'PUT') {
      routineState.current = JSON.parse(String(init?.body)).routine_id
      routineState.puts.push(routineState.current!)
    }
    return json({ routine_id: routineState.current })
  }
  if (url.endsWith('/routines') && method === 'GET') return json(routineState.routines)
  return mockFetch(url, init)
}
beforeEach(() => {
  routineState = { current: 'r1', routines: [routineOf('r1', 'Split', ['w1'])], puts: [] }
  vi.stubGlobal('fetch', routineFetch)
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
  const workout = { id: 'w1', routine_id: 'r1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
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
    const workout = { id: 'w1', routine_id: 'r1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
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
    const workout = { id: 'w1', routine_id: 'r1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
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
  const workout = { id: 'w1', routine_id: 'r1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
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
    const workout = { id: 'w1', routine_id: 'r1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
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
  const workout = { id: 'w1', routine_id: 'r1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
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

describe('App — rest timer', () => {
  const workout = { id: 'w1', routine_id: 'r1', name: 'Push', exercises: [], created_at: '', updated_at: '' }
  const mkEx = (id: string, name: string, rest: number) => ({
    id, exercise_id: '', movement_id: `m-${id}`, name, rest_seconds: rest, exercise: { id: '', name },
    sets: [{ id: `${id}-1`, weight: 100, reps: 8, completed: false }], previous: [],
  })
  const calls: string[] = []

  beforeEach(() => {
    localStorage.removeItem('liftoff-rest-timer')
    calls.length = 0
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      const json = (body: unknown, status = 200) => Promise.resolve({ ok: true, status, json: () => Promise.resolve(body) })
      const method = init?.method ?? 'GET'
      if (url.includes('/sessions/active')) return json({ id: 's1', workout_id: 'w1', workout, started_at: '', is_active: true, exercises: [mkEx('a', 'Bench', 90), mkEx('b', 'Plank', 0)] })
      if (url.endsWith('/workouts')) return json([workout])
      if (url.match(/\/exercise-sets\/\w+-1$/) && method === 'PATCH') return json({ id: 'x', weight: 100, reps: 8, completed: true })
      const mv = url.match(/\/movements\/([\w-]+)$/)
      if (mv && method === 'PATCH') { calls.push(`rest ${mv[1]} ${init?.body}`); return json(undefined, 204) }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
    })
  })

  const open = async () => {
    renderWithAuth(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Continue Session' }))
    await screen.findByRole('button', { name: 'Set 1 done', hidden: false }).catch(() => null)
  }

  test('ticking a set starts that exercise\'s rest; an exercise with rest off starts nothing', async () => {
    await open()
    fireEvent.click((await screen.findAllByRole('button', { name: 'Set 1 done' }))[1]) // Plank: off
    expect(screen.queryByRole('timer')).toBeNull()
    fireEvent.click(screen.getAllByRole('button', { name: 'Set 1 done' })[0]) // Bench: 90 s
    const bar = await screen.findByRole('timer')
    expect(bar).toHaveTextContent('1:30')
    expect(bar).toHaveTextContent('Bench')
  })

  test('a set that fails to save does not start the rest timer', async () => {
    const ok = mockFetch.getMockImplementation()!
    mockFetch.mockImplementation((url: string, init?: RequestInit) =>
      url.match(/\/exercise-sets\/\w+-1$/) && init?.method === 'PATCH'
        ? Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({}) })
        : ok(url, init))
    await open()
    fireEvent.click((await screen.findAllByRole('button', { name: 'Set 1 done' }))[0])
    await screen.findByText('Failed to update set')
    expect(screen.queryByRole('timer')).toBeNull()
  })

  test('rest times read the same in the menu and the dialog, including unlisted values', () => {
    expect([0, 45, 90, 120, 75, 150].map(formatRest)).toEqual(['Off', '45 s', '90 s', '2 min', '75 s', '2 min 30 s'])
  })

  test('the rest setting is saved for the exercise and shown in the menu', async () => {
    await open()
    fireEvent.click(await screen.findByRole('button', { name: 'Plank options' }))
    fireEvent.click(screen.getByRole('menuitem', { name: /Rest timer: off/ }))
    fireEvent.click(screen.getByRole('button', { name: '2 min' }))
    await waitFor(() => expect(calls).toEqual(['rest m-b {"rest_seconds":120}']))
    fireEvent.click(screen.getByRole('button', { name: 'Plank options' }))
    expect(screen.getByRole('menuitem', { name: /Rest timer: 2 min/ })).toBeInTheDocument()
  })
})


describe('App — error popup', () => {
  beforeEach(() => {
    mockFetch.mockImplementation(() => Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({}) }))
  })

  test('an error shows as an alert popup and can be dismissed', async () => {
    renderWithAuth(<App />)
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Failed to load workouts')
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss error' }))
    expect(screen.queryByRole('alert')).toBeNull()
  })

  test('the error clears when you switch tabs', async () => {
    renderWithAuth(<App />)
    await screen.findByRole('alert')
    fireEvent.click(screen.getAllByRole('button', { name: 'Routines' })[0])
    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull())
  })

  test('a failed progress load shows inline on the Progress tab, not as a toast', async () => {
    mockFetch.mockImplementation((url: string) => url.includes('/workouts')
      ? Promise.resolve({ ok: true, json: () => Promise.resolve([]) })
      : Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({}) }))
    renderWithAuth(<App />)
    await screen.findByRole('button', { name: 'Create' })
    fireEvent.click(screen.getAllByRole('button', { name: 'Progress' })[0])
    expect(await screen.findByText('Failed to load progress data')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.queryByText(/No progress data yet/)).toBeNull()
  })
})

describe('App — workout editor', () => {
  const ex = (id: string, name: string, weight = 100) => ({ id, name, sets: 3, reps: 10, weight, workout_id: 'w1', created_at: '', updated_at: '' })
  let exercises: ReturnType<typeof ex>[]
  const calls: string[] = []

  beforeEach(() => {
    calls.length = 0
    exercises = [ex('e1', 'Bench'), ex('e2', 'Row')]
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      const json = (body: unknown, status = 200) => Promise.resolve({ ok: true, status, json: () => Promise.resolve(body) })
      const method = init?.method ?? 'GET'
      const w = { id: 'w1', routine_id: 'r1', name: 'Push', exercises, created_at: '', updated_at: '' }
      if (url.includes('/sessions/active')) return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
      if (url.endsWith('/workouts')) return json([w])
      if (url.endsWith('/workouts/w1/exercises')) return json(exercises)
      if (url.endsWith('/movements') && method === 'GET') return json([{ id: 'm-1', name: 'Squat', category: 'legs', last_used: null }])
      if (url.endsWith('/exercise-templates')) return json([{ name: 'Deadlift', category: 'back', default_sets: 5, default_reps: 5, default_weight: 135 }])
      if (url.endsWith('/exercises') && method === 'POST') {
        calls.push('create ' + init?.body)
        return json(ex('e3', JSON.parse(String(init?.body)).name), 201)
      }
      const one = url.match(/\/exercises\/(\w+)$/)
      if (one && method === 'PATCH') {
        calls.push(`patch ${one[1]} ${init?.body}`)
        return json({ ...exercises.find(e => e.id === one[1])!, ...JSON.parse(String(init?.body)) })
      }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
    })
  })

  const open = async () => {
    renderWithAuth(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Push' }))
    await screen.findByText('Current Workout: Push')
  }

  const names = () => [...document.querySelectorAll('.exercise-card h4')].map(h => h.textContent)

  test('clicking a workout card opens its plan in the editor, with no session started', async () => {
    renderWithAuth(<App />)
    const select = await screen.findByRole('button', { name: 'Push' })
    expect(screen.queryByText('Current Workout: Push')).toBeNull()
    expect(select).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(select.closest('.workout-card') as HTMLElement) // anywhere on the card
    await screen.findByText('Current Workout: Push')
    expect(screen.getByText('Bench')).toBeInTheDocument()
    expect(select).toHaveAttribute('aria-pressed', 'true')
    expect(select.closest('.workout-card')).toHaveClass('selected')
    expect(calls).toEqual([]) // selecting doesn't hit the server
  })

  test('shows the plan and has no set logging', async () => {
    await open()
    expect(screen.getAllByText('3 × 10 @ 100 lbs')).toHaveLength(2)
    expect(screen.queryByText(/log set/i)).toBeNull()
  })

  test('adds an exercise through the popup with library defaults', async () => {
    await open()
    expect(screen.queryByText('Quick Add Exercise')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: '+ Add exercise' }))
    fireEvent.click(await screen.findByText('Deadlift'))
    expect(await screen.findByRole('heading', { name: 'Deadlift' })).toBeInTheDocument()
    expect(calls).toEqual(['create {"name":"Deadlift","sets":5,"reps":5,"weight":135,"workout_id":"w1"}'])
    expect(screen.queryByRole('dialog', { name: 'Add exercise' })).toBeNull()
  })

  test('creates a custom exercise by typing a name', async () => {
    await open()
    fireEvent.click(screen.getByRole('button', { name: '+ Add exercise' }))
    fireEvent.change(await screen.findByPlaceholderText('Search or type a new exercise'), { target: { value: 'Face Pull' } })
    fireEvent.click(screen.getByText('Create “Face Pull”'))
    await screen.findByRole('heading', { name: 'Face Pull' })
    expect(calls).toEqual(['create {"name":"Face Pull","sets":3,"reps":10,"weight":0,"workout_id":"w1"}'])
  })

  test('edits a planned value and sends only what changed', async () => {
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Edit Bench' }))
    fireEvent.change(screen.getAllByLabelText('Sets')[0], { target: { value: '5' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(await screen.findByText('5 × 10 @ 100 lbs')).toBeInTheDocument()
    expect(calls).toEqual(['patch e1 {"sets":5}'])
  })

  test('fractional sets disable Save', async () => {
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Edit Bench' }))
    fireEvent.change(screen.getAllByLabelText('Sets')[0], { target: { value: '3.5' } })
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })

  test('move down reorders and tells the server the position', async () => {
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Move Bench down' }))
    await waitFor(() => expect(calls).toEqual(['patch e1 {"position":1}']))
    await waitFor(() => expect(names()).toEqual(['Row', 'Bench']))
  })

  test('a move and an edit in flight together both stick', async () => {
    let release = () => {}
    const base = mockFetch.getMockImplementation()!
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.endsWith('/exercises/e1') && init?.method === 'PATCH' && String(init.body).includes('position')) {
        calls.push(`patch e1 ${init.body}`)
        return new Promise(resolve => { release = () => resolve({ ok: true, status: 200, json: () => Promise.resolve(exercises[0]) }) })
      }
      return base(url, init)
    })
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Move Bench down' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Move Row up' })).toBeDisabled()) // one move at a time
    fireEvent.click(screen.getByRole('button', { name: 'Edit Row' }))
    fireEvent.change(screen.getByLabelText('Sets'), { target: { value: '5' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(await screen.findByText('5 × 10 @ 100 lbs')).toBeInTheDocument()
    release()
    await waitFor(() => expect(names()).toEqual(['Row', 'Bench']))
    expect(screen.getByText('5 × 10 @ 100 lbs')).toBeInTheDocument()
  })
})

describe('App — deleting a routine', () => {
  const wk = (id: string, name: string, routine_id: string) => ({ id, routine_id, name, exercises: [], created_at: '', updated_at: '' })
  let workoutsOnServer: ReturnType<typeof wk>[]
  let deleted: string[]

  beforeEach(() => {
    deleted = []
    workoutsOnServer = [wk('w1', 'Push', 'r1'), wk('w2', 'Pull', 'r1'), wk('w3', 'Solo', 'r2')]
    routineState.routines = [routineOf('r1', 'Split', ['w1', 'w2']), routineOf('r2', 'Other', ['w3'])]
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      const json = (body: unknown, status = 200) => Promise.resolve({ ok: true, status, json: () => Promise.resolve(body) })
      const method = init?.method ?? 'GET'
      const del = url.match(/\/routines\/(\w+)$/)
      if (del && method === 'DELETE') {
        deleted.push(del[1])
        workoutsOnServer = workoutsOnServer.filter(w => w.routine_id !== del[1])
        routineState.current = 'r2' // the server falls back to another routine
        return json({ message: 'ok' })
      }
      if (url.endsWith('/workouts')) return json(workoutsOnServer)
      if (url.match(/\/workouts\/\w+\/exercises$/)) return json([])
      if (url.endsWith('/sessions/active')) return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
      return json([])
    })
  })

  test('the confirm names the routine and its workouts; the Workouts tab refreshes after', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderWithAuth(<App />)
    await screen.findByText('Push')
    expect(screen.queryByText('Solo')).toBeNull() // another routine's workout

    fireEvent.click(screen.getAllByRole('button', { name: 'Routines' })[0])
    const buttons = await screen.findAllByRole('button', { name: '×' })
    fireEvent.click(buttons[0])

    expect(confirm).toHaveBeenCalledOnce()
    const text = confirm.mock.calls[0][0] as string
    expect(text).toContain('"Split"')
    expect(text).toContain('2 workouts will be deleted')
    expect(text).toContain('history is kept')
    await waitFor(() => expect(deleted).toEqual(['r1']))

    fireEvent.click(screen.getAllByRole('button', { name: 'Workouts' })[0])
    await screen.findByText('Solo') // r2 is current now
    expect(screen.getByRole('heading', { name: 'Other' })).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByText('Push')).toBeNull())
    expect(screen.queryByText('Pull')).toBeNull()
    confirm.mockRestore()
  })

  test('declining the confirm deletes nothing', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    renderWithAuth(<App />)
    await screen.findByText('Push')
    fireEvent.click(screen.getAllByRole('button', { name: 'Routines' })[0])
    fireEvent.click((await screen.findAllByRole('button', { name: '×' }))[0])
    expect(confirm).toHaveBeenCalledOnce()
    expect(deleted).toEqual([])
    confirm.mockRestore()
  })
})

describe('App — current routine on the Workouts tab', () => {
  const wk = (id: string, name: string, routine_id: string) => ({ id, routine_id, name, exercises: [], created_at: '', updated_at: '' })
  let workoutsOnServer: ReturnType<typeof wk>[]
  let created: unknown[]

  beforeEach(() => {
    created = []
    workoutsOnServer = [wk('w1', 'Push', 'r1'), wk('w2', 'Pull', 'r1'), wk('w3', 'Solo', 'r2')]
    routineState.routines = [routineOf('r1', 'Split', ['w1', 'w2']), routineOf('r2', 'Other', ['w3'])]
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      const json = (body: unknown, status = 200) => Promise.resolve({ ok: true, status, json: () => Promise.resolve(body) })
      const method = init?.method ?? 'GET'
      if (url.endsWith('/workouts') && method === 'POST') {
        const body = JSON.parse(String(init?.body))
        created.push(body)
        workoutsOnServer.push(wk('w9', body.name, body.routine_id))
        return json(workoutsOnServer[workoutsOnServer.length - 1], 201)
      }
      if (url.endsWith('/workouts')) return json(workoutsOnServer)
      if (url.endsWith('/sessions/active')) return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) })
      return json([])
    })
  })

  const goTo = (name: string) => fireEvent.click(screen.getAllByRole('button', { name })[0])

  test('the heading is the current routine, and only its workouts are listed', async () => {
    renderWithAuth(<App />)
    expect(await screen.findByRole('heading', { name: 'Split' })).toBeInTheDocument()
    expect(screen.queryByText('Your Workouts')).toBeNull()
    expect(screen.getByText('Workouts in this routine')).toBeInTheDocument()
    expect(screen.getByText('Push')).toBeInTheDocument()
    expect(screen.getByText('Pull')).toBeInTheDocument()
    expect(screen.queryByText('Solo')).toBeNull()
  })

  test('switching routines saves it and changes the list', async () => {
    renderWithAuth(<App />)
    await screen.findByText('Push')
    fireEvent.change(screen.getByLabelText('Switch routine'), { target: { value: 'r2' } })
    expect(await screen.findByText('Solo')).toBeInTheDocument()
    expect(routineState.puts).toEqual(['r2'])
    expect(screen.getByRole('heading', { name: 'Other' })).toBeInTheDocument()
    expect(screen.queryByText('Push')).toBeNull()
  })

  test('switching clears a selected workout from the old routine', async () => {
    renderWithAuth(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Push' }))
    await screen.findByText('Current Workout: Push')
    fireEvent.change(screen.getByLabelText('Switch routine'), { target: { value: 'r2' } })
    await screen.findByText('Solo')
    expect(screen.queryByText('Current Workout: Push')).toBeNull()
  })

  test('Create New Workout sends the current routine id', async () => {
    renderWithAuth(<App />)
    await screen.findByText('Push')
    fireEvent.change(screen.getByLabelText('Switch routine'), { target: { value: 'r2' } })
    await screen.findByText('Solo')
    fireEvent.change(screen.getByPlaceholderText('Workout name...'), { target: { value: 'Legs' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await screen.findByText('Legs')
    expect(created).toEqual([{ name: 'Legs', routine_id: 'r2' }])
  })

  test('with no routines, says to create a workout or pick a template', async () => {
    routineState.routines = []
    routineState.current = null
    workoutsOnServer = []
    renderWithAuth(<App />)
    expect(await screen.findByText(/No routine yet/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Switch routine')).toBeNull()
  })

  test('a current routine with no workouts says so', async () => {
    routineState.routines = [routineOf('r1', 'Split'), routineOf('r2', 'Other', ['w3'])]
    workoutsOnServer = [wk('w3', 'Solo', 'r2')]
    renderWithAuth(<App />)
    expect(await screen.findByText(/No workouts in this routine yet/)).toBeInTheDocument()
  })

  test('the Routines tab badges the current routine and can switch to another', async () => {
    renderWithAuth(<App />)
    await screen.findByText('Push')
    goTo('Routines')
    expect(await screen.findByText('Current')).toBeInTheDocument()
    const use = screen.getAllByRole('button', { name: 'Use this routine' })
    expect(use).toHaveLength(1) // not offered on the current one
    fireEvent.click(use[0])
    await waitFor(() => expect(routineState.puts).toEqual(['r2']))
    await waitFor(() => expect(screen.getByRole('heading', { name: /Other/ })).toHaveTextContent('Current'))
    expect(screen.getByRole('heading', { name: /Split/ })).not.toHaveTextContent('Current')
    fireEvent.click(screen.getAllByRole('button', { name: 'Workouts' })[0])
    expect(await screen.findByRole('heading', { name: 'Other' })).toBeInTheDocument()
    expect(screen.getByText('Solo')).toBeInTheDocument()
  })
})
