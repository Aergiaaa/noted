import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { RouterProvider, createMemoryRouter } from 'react-router-dom'
import { routes } from '../../app/routes'

afterEach(() => {
  cleanup()
})

function renderAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  render(<RouterProvider router={router} />)
  return router
}

describe('AppShell', () => {
  it('renders brand plus all five nav links', () => {
    renderAt('/')
    expect(screen.getByText('Noted')).toBeDefined()
    for (const label of ['Dashboard', 'Calendar', 'Tasks', 'Notes', 'Finance']) {
      expect(screen.getByRole('link', { name: label })).toBeDefined()
    }
  })

  it('marks only the current route active (both className branches)', () => {
    renderAt('/')
    // Dashboard is exact (end:true) so it is active at / …
    expect(screen.getByRole('link', { name: 'Dashboard' }).className).toContain(
      'btn-active',
    )
    // … while every other link takes the inactive branch.
    for (const label of ['Calendar', 'Tasks', 'Notes', 'Finance']) {
      expect(screen.getByRole('link', { name: label }).className).not.toContain(
        'btn-active',
      )
    }
  })

  it('navigates via nav click and moves the active state', async () => {
    renderAt('/')
    fireEvent.click(screen.getByRole('link', { name: 'Calendar' }))

    await screen.findByRole('heading', { name: 'Calendar' })
    expect(screen.getByRole('link', { name: 'Calendar' }).className).toContain(
      'btn-active',
    )
    // end:true keeps Dashboard inactive on sub-routes.
    expect(screen.getByRole('link', { name: 'Dashboard' }).className).not.toContain(
      'btn-active',
    )
  })

  it('renders the outlet content for deep links', async () => {
    renderAt('/tasks')
    await screen.findByRole('heading', { name: 'Tasks' })
  })

  it('redirects unknown paths back to the dashboard', async () => {
    renderAt('/does-not-exist')
    await screen.findByRole('heading', { name: 'Dashboard' })
  })

  it('renders /login outside the shell (no nav chrome)', () => {
    renderAt('/login')
    expect(screen.getByRole('heading', { name: 'Login' })).toBeDefined()
    expect(screen.queryByText('Noted')).toBeNull()
  })

  it('recovers via retry-free navigation after a redirect', async () => {
    const router = renderAt('/nope')
    await screen.findByRole('heading', { name: 'Dashboard' })
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/')
    })
  })
})
