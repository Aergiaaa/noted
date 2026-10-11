import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { App } from './router'
import { SessionProvider } from '../lib/session'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  window.history.replaceState(null, '', '/')
})

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function stubProbe(fetchMock: ReturnType<typeof vi.fn>) {
  vi.stubGlobal('fetch', fetchMock)
}

function stubAuthed(extra: Array<[string, Response]> = []) {
  const fetchMock = vi
    .fn()
    .mockResolvedValueOnce(jsonResponse(200, { authenticated: true, expires_at: 'x' }))
  for (const [url, res] of extra) {
    fetchMock.mockResolvedValueOnce(res).mockResolvedValue(res)
    void url
  }
  stubProbe(fetchMock)
  return fetchMock
}

function stubGuest() {
  stubProbe(
    vi.fn().mockResolvedValue(
      jsonResponse(401, { error: { code: 'unauthenticated', message: 'nope' } }),
    ),
  )
}

function renderApp() {
  return render(
    <SessionProvider>
      <App />
    </SessionProvider>,
  )
}

describe('App guard', () => {
  it('shows a skeleton while the session probe is in flight', () => {
    // A never-resolving fetch keeps status at "loading".
    vi.stubGlobal('fetch', vi.fn().mockReturnValue(new Promise<Response>(() => {})))

    renderApp()

    expect(screen.getByRole('status', { name: 'Loading' })).toBeDefined()
  })

  it('offers retry when the probe fails, then proceeds', async () => {
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new TypeError('offline'))
      .mockResolvedValue(jsonResponse(200, { authenticated: true, expires_at: 'x' }))
    stubProbe(fetchMock)

    renderApp()
    await waitFor(() => expect(screen.getByRole('alert')).toBeDefined())

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    })

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Dashboard' })).toBeDefined())
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('sends guests to /login, replacing the history entry', async () => {
    window.history.replaceState(null, '', '/tasks')
    stubGuest()

    renderApp()

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Login' })).toBeDefined())
    expect(window.location.pathname).toBe('/login')
  })

  it('lets guests sit on /login', async () => {
    window.history.replaceState(null, '', '/login')
    stubGuest()

    renderApp()

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Login' })).toBeDefined())
    expect(window.location.pathname).toBe('/login')
  })

  it('wraps module routes in the shell for authed users', async () => {
    window.history.replaceState(null, '', '/calendar')
    stubAuthed()

    renderApp()

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Calendar' })).toBeDefined())
    expect(screen.getByText('Noted')).toBeDefined()
    expect(window.location.pathname).toBe('/calendar')
  })

  it('redirects authed users off /login to the dashboard', async () => {
    window.history.replaceState(null, '', '/login')
    stubAuthed()

    renderApp()

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Dashboard' })).toBeDefined())
    expect(window.location.pathname).toBe('/')
  })

  it('redirects authed users from unknown paths to /', async () => {
    window.history.replaceState(null, '', '/missing-page')
    stubAuthed()

    renderApp()

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Dashboard' })).toBeDefined())
    expect(window.location.pathname).toBe('/')
  })

  it('renders /login outside the shell', async () => {
    window.history.replaceState(null, '', '/login')
    stubGuest()

    renderApp()

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Login' })).toBeDefined())
    expect(screen.queryByText('Noted')).toBeNull()
  })
})
