import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import { SessionProvider, useSession } from './session'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

// Probe status rendered so assertions don't depend on consumer pages;
// login/logout errors are surfaced in the DOM because callers catch them.
function Probe() {
  const { status, login, logout, retry } = useSession()
  const [lastError, setLastError] = useState('none')
  return (
    <div>
      <span data-testid="status">{status}</span>
      <span data-testid="last-error">{lastError}</span>
      <button
        onClick={() => {
          login('123456').catch((e: unknown) => setLastError((e as Error).message))
        }}
      >
        in
      </button>
      <button
        onClick={() => {
          logout().catch((e: unknown) => setLastError((e as Error).message))
        }}
      >
        out
      </button>
      <button onClick={retry}>again</button>
    </div>
  )
}

function renderProvider() {
  return render(
    <SessionProvider>
      <Probe />
    </SessionProvider>,
  )
}

function status() {
  return screen.getByTestId('status').textContent
}

describe('SessionProvider', () => {
  it('probes GET /api/session once and lands authed on 200', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(200, { authenticated: true, expires_at: '2026-11-01T00:00:00Z' }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderProvider()

    expect(status()).toBe('loading')
    await waitFor(() => expect(status()).toBe('authed'))
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock).toHaveBeenCalledWith('/api/session', expect.anything())
  })

  it('lands guest on 401 without treating it as an error', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(401, { error: { code: 'unauthenticated', message: 'nope' } }),
      ),
    )

    renderProvider()

    await waitFor(() => expect(status()).toBe('guest'))
  })

  it('lands error on network failure so the guard can offer retry', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('offline')))

    renderProvider()

    await waitFor(() => expect(status()).toBe('error'))
  })

  it('lands error when the probe 200 body cannot be decoded', async () => {
    // Not an ApiError: the guard must still route it to "error", not crash.
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('not json', { status: 200 })))

    renderProvider()

    await waitFor(() => expect(status()).toBe('error'))
  })

  it('ignores a probe that resolves after unmount', async () => {
    let resolveFetch!: (r: Response) => void
    vi.stubGlobal(
      'fetch',
      vi.fn().mockReturnValue(new Promise<Response>((res) => { resolveFetch = res })),
    )

    const { unmount } = renderProvider()
    unmount()
    resolveFetch(jsonResponse(200, { authenticated: true }))
    await new Promise((r) => setTimeout(r, 0))
  })

  it('ignores a probe that rejects after unmount', async () => {
    let rejectFetch!: (e: unknown) => void
    vi.stubGlobal(
      'fetch',
      vi.fn().mockReturnValue(new Promise<Response>((_res, rej) => { rejectFetch = rej })),
    )

    const { unmount } = renderProvider()
    unmount()
    rejectFetch(new TypeError('offline'))
    await new Promise((r) => setTimeout(r, 0))
  })

  it('re-probes when retry is clicked after an error', async () => {
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new TypeError('offline'))
      .mockResolvedValue(jsonResponse(200, { authenticated: true, expires_at: 'x' }))
    vi.stubGlobal('fetch', fetchMock)

    renderProvider()
    await waitFor(() => expect(status()).toBe('error'))

    await act(async () => {
      screen.getByText('again').click()
    })

    await waitFor(() => expect(status()).toBe('authed'))
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('login posts the code and flips to authed on success', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: 'unauthenticated', message: 'nope' } }),
      )
      .mockResolvedValueOnce(jsonResponse(200, { ok: true }))
    vi.stubGlobal('fetch', fetchMock)

    renderProvider()
    await waitFor(() => expect(status()).toBe('guest'))

    await act(async () => {
      screen.getByText('in').click()
    })

    expect(status()).toBe('authed')
    expect(fetchMock).toHaveBeenCalledWith('/api/auth/login', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ code: '123456' }),
    })
  })

  it('login leaves status guest and rethrows for the page to render', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() =>
        Promise.resolve(
          jsonResponse(401, { error: { code: 'unauthenticated', message: 'invalid code' } }),
        ),
      ),
    )

    renderProvider()
    await waitFor(() => expect(status()).toBe('guest'))

    await act(async () => {
      screen.getByText('in').click()
    })

    expect(status()).toBe('guest')
    expect(screen.getByTestId('last-error').textContent).toBe('invalid code')
  })

  it('logout posts and flips to guest', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(200, { authenticated: true, expires_at: 'x' }))
      .mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)

    renderProvider()
    await waitFor(() => expect(status()).toBe('authed'))

    await act(async () => {
      screen.getByText('out').click()
    })

    expect(status()).toBe('guest')
    expect(fetchMock).toHaveBeenLastCalledWith('/api/auth/logout', {
      method: 'POST',
      credentials: 'include',
      headers: {},
      body: undefined,
    })
  })

  it('logout failure keeps the session and surfaces the error', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValueOnce(jsonResponse(200, { authenticated: true, expires_at: 'x' }))
        .mockRejectedValueOnce(new TypeError('offline')),
    )

    renderProvider()
    await waitFor(() => expect(status()).toBe('authed'))

    await act(async () => {
      screen.getByText('out').click()
    })

    expect(status()).toBe('authed')
    expect(screen.getByTestId('last-error').textContent).toBe('network request failed')
  })

  it('useSession outside a provider throws', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    function Orphan() {
      useSession()
      return null
    }
    expect(() => render(<Orphan />)).toThrow('useSession must be used within SessionProvider')
    spy.mockRestore()
  })
})
