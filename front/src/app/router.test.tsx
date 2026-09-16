import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { RouterProvider } from 'react-router-dom'
import { router } from './router'

afterEach(() => {
  cleanup()
})

// The singleton browser router is what main.tsx boots. These tests pin
// its table and prove every path renders through the real router.
describe('router', () => {
  it('holds the login, shell, and fallback routes', () => {
    expect(router.routes).toHaveLength(3)
    expect(router.routes.map((r) => r.path)).toEqual(['/login', '/', '*'])
  })

  it('renders the dashboard at the root entry', async () => {
    render(<RouterProvider router={router} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
  })

  it('reaches every module route', async () => {
    render(<RouterProvider router={router} />)
    for (const [path, heading] of [
      ['/calendar', 'Calendar'],
      ['/tasks', 'Tasks'],
      ['/notes', 'Notes'],
      ['/finance', 'Finance'],
      ['/login', 'Login'],
    ] as const) {
      await router.navigate(path)
      await screen.findByRole('heading', { name: heading })
    }
  })

  it('sends unknown routes back to /', async () => {
    render(<RouterProvider router={router} />)
    await router.navigate('/missing-page')
    await screen.findByRole('heading', { name: 'Dashboard' })
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/')
    })
  })
})
