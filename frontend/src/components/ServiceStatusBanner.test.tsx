import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ServiceStatusBanner } from './ServiceStatusBanner'

const respond = (status: number, body: object) =>
  vi.fn().mockResolvedValue({ ok: status < 400, status, json: () => Promise.resolve(body) })

describe('ServiceStatusBanner', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('shows the server message when the database is unavailable', async () => {
    vi.stubGlobal('fetch', respond(503, { database: 'unavailable', error: 'Liftoff can\'t reach its database right now.' }))
    render(<ServiceStatusBanner />)
    expect(await screen.findByRole('alert')).toHaveTextContent("can't reach its database")
  })

  it('shows a connection message when the server is unreachable', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    render(<ServiceStatusBanner />)
    expect(await screen.findByRole('alert')).toHaveTextContent("Can't reach the Liftoff server")
  })

  it('renders nothing when everything is up', async () => {
    const fetchMock = respond(200, { database: 'ok' })
    vi.stubGlobal('fetch', fetchMock)
    render(<ServiceStatusBanner />)
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalled())
    expect(screen.queryByRole('alert')).toBeNull()
  })
})
