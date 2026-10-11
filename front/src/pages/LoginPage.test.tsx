import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import LoginPage from './LoginPage'
import { SessionProvider } from '../lib/session'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  })
}

function renderLogin() {
  return render(
    <SessionProvider>
      <LoginPage />
    </SessionProvider>,
  )
}

function submitCode(code: string) {
  fireEvent.change(screen.getByLabelText('Code'), { target: { value: code } })
  fireEvent.click(screen.getByRole('button', { name: 'Log in' }))
}

// Probe answers guest so LoginPage mounts the way the guard leaves it.
function stubProbe() {
  return vi
    .fn()
    .mockResolvedValueOnce(
      jsonResponse(401, { error: { code: 'unauthenticated', message: 'nope' } }),
    )
}

describe('LoginPage', () => {
  it('shows a client-side required error and skips the network on empty code', async () => {
    const fetchMock = stubProbe()
    vi.stubGlobal('fetch', fetchMock)

    renderLogin()
    await waitFor(() => expect(screen.getByLabelText('Code')).toBeDefined())

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Log in' }))
    })

    expect(screen.getByText('Enter the 6-digit code or a recovery code.')).toBeDefined()
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('submits the code and leaves the form in a logged-in world', async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(jsonResponse(200, { ok: true }))
    vi.stubGlobal('fetch', fetchMock)

    renderLogin()
    await waitFor(() => expect(screen.getByLabelText('Code')).toBeDefined())

    await act(async () => {
      submitCode('123456')
    })

    expect(fetchMock).toHaveBeenLastCalledWith('/api/auth/login', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ code: '123456' }),
    })
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('shows the invalid-code message on 401', async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(401, { error: { code: 'unauthenticated', message: 'invalid code' } }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderLogin()
    await waitFor(() => expect(screen.getByLabelText('Code')).toBeDefined())

    await act(async () => {
      submitCode('000000')
    })

    expect(screen.getByRole('alert').textContent).toBe('Invalid code. Try again.')
  })

  it('shows the rate-limit countdown from Retry-After on 429', async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(
        429,
        { error: { code: 'rate_limited', message: 'slow down' } },
        { 'Retry-After': '42' },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderLogin()
    await waitFor(() => expect(screen.getByLabelText('Code')).toBeDefined())

    await act(async () => {
      submitCode('123456')
    })

    expect(screen.getByRole('alert').textContent).toBe('Too many attempts. Try again in 42s.')
  })

  it('falls back to a generic rate-limit message without Retry-After', async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(429, { error: { code: 'rate_limited', message: 'slow down' } }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderLogin()
    await waitFor(() => expect(screen.getByLabelText('Code')).toBeDefined())

    await act(async () => {
      submitCode('123456')
    })

    expect(screen.getByRole('alert').textContent).toBe('Too many attempts. Try again later.')
  })

  it('shows a network message when the login call fails', async () => {
    const fetchMock = stubProbe().mockRejectedValueOnce(new TypeError('offline'))
    vi.stubGlobal('fetch', fetchMock)

    renderLogin()
    await waitFor(() => expect(screen.getByLabelText('Code')).toBeDefined())

    await act(async () => {
      submitCode('123456')
    })

    expect(screen.getByRole('alert').textContent).toBe(
      'Could not reach the server. Check the connection and retry.',
    )
  })

  it('shows a generic message on unexpected server errors', async () => {
    const fetchMock = stubProbe().mockResolvedValueOnce(
      jsonResponse(500, { error: { code: 'internal_error', message: 'boom' } }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderLogin()
    await waitFor(() => expect(screen.getByLabelText('Code')).toBeDefined())

    await act(async () => {
      submitCode('123456')
    })

    expect(screen.getByRole('alert').textContent).toBe('Login failed. Try again.')
  })

  it('disables the button while the login call is in flight', async () => {
    let resolveLogin!: (r: Response) => void
    const fetchMock = stubProbe().mockImplementationOnce(
      () => new Promise<Response>((res) => { resolveLogin = res }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderLogin()
    await waitFor(() => expect(screen.getByLabelText('Code')).toBeDefined())

    await act(async () => {
      submitCode('123456')
    })

    const button = screen.getByRole('button', { name: 'Checking…' })
    expect(button).toHaveProperty('disabled', true)

    await act(async () => {
      resolveLogin(jsonResponse(200, { ok: true }))
    })
  })
})
