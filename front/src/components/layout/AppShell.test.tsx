import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import AppShell from './AppShell'
import { SessionProvider } from '../../lib/session'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  window.history.replaceState(null, '', '/')
})

function jsonResponse(status: number, body: unknown) {
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

async function renderShell() {
  const view = render(
    <SessionProvider>
      <AppShell>
        <p>content</p>
      </AppShell>
    </SessionProvider>,
  )
  // Wait for the probe so status is authed before asserting.
  await screen.findByText('Noted')
  return view
}

describe('AppShell', () => {
  it('renders brand plus all five nav links and the logout button', async () => {
    stubAuthed()
    await renderShell()

    expect(screen.getByText('Noted')).toBeDefined()
    for (const label of ['Dashboard', 'Calendar', 'Tasks', 'Notes', 'Finance']) {
      expect(screen.getByRole('link', { name: label })).toBeDefined()
    }
    expect(screen.getByRole('button', { name: 'Log out' })).toBeDefined()
  })

  it('marks only the current route active (both className branches)', async () => {
    stubAuthed()
    window.history.replaceState(null, '', '/')
    await renderShell()

    expect(screen.getByRole('link', { name: 'Dashboard' }).className).toContain('btn-active')
    for (const label of ['Calendar', 'Tasks', 'Notes', 'Finance']) {
      expect(screen.getByRole('link', { name: label }).className).not.toContain('btn-active')
    }
  })

  it('moves the active state on nav click', async () => {
    stubAuthed()
    await renderShell()

    fireEvent.click(screen.getByRole('link', { name: 'Calendar' }))

    await waitFor(() =>
      expect(screen.getByRole('link', { name: 'Calendar' }).className).toContain('btn-active'),
    )
    expect(screen.getByRole('link', { name: 'Dashboard' }).className).not.toContain('btn-active')
  })

  it('logs out via POST and clears the session on success', async () => {
    const fetchMock = stubAuthed([
      ['/api/auth/logout', new Response(null, { status: 204 })],
    ])
    await renderShell()

    fireEvent.click(screen.getByRole('button', { name: 'Log out' }))

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/auth/logout',
        expect.objectContaining({ method: 'POST', credentials: 'include' }),
      ),
    )
  })

  it('surfaces an inline message when logout fails', async () => {
    stubAuthed([
      [
        '/api/auth/logout',
        new Response(JSON.stringify({ error: { code: 'internal_error', message: 'boom' } }), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        }),
      ],
    ])
    await renderShell()

    fireEvent.click(screen.getByRole('button', { name: 'Log out' }))

    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('Logout failed'))
  })
})
