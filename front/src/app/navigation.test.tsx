import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { A, isActivePath, navigate, usePath } from './navigation'

afterEach(() => {
  cleanup()
  window.history.replaceState(null, '', '/')
})

function PathProbe() {
  const path = usePath()
  return <span data-testid="path">{path}</span>
}

function currentPath() {
  return screen.getByTestId('path').textContent
}

describe('usePath', () => {
  it('reports the initial location', () => {
    window.history.replaceState(null, '', '/tasks?due=2026-10-11')
    render(<PathProbe />)
    expect(currentPath()).toBe('/tasks?due=2026-10-11')
  })

  it('re-renders when navigate() pushes a new path', () => {
    render(<PathProbe />)
    act(() => {
      navigate('/calendar')
    })
    expect(currentPath()).toBe('/calendar')
    expect(window.location.pathname).toBe('/calendar')
  })

  it('re-renders on browser back/forward (popstate)', () => {
    render(<PathProbe />)
    act(() => {
      window.history.pushState(null, '', '/notes')
      window.dispatchEvent(new PopStateEvent('popstate'))
    })
    expect(currentPath()).toBe('/notes')
  })

  it('removes its listeners on unmount', () => {
    const add = vi.spyOn(window, 'addEventListener')
    const remove = vi.spyOn(window, 'removeEventListener')
    const { unmount } = render(<PathProbe />)
    unmount()

    const tracked = (spy: typeof add) =>
      spy.mock.calls.filter(([type]) => type === 'popstate' || type === 'noted:navigate')
    expect(tracked(add)).toHaveLength(2)
    expect(tracked(remove)).toHaveLength(2)
    add.mockRestore()
    remove.mockRestore()
  })
})

describe('navigate', () => {
  it('replaces instead of pushing when asked, keeping history clean', () => {
    render(<PathProbe />)
    const replaceSpy = vi.spyOn(window.history, 'replaceState')
    act(() => {
      navigate('/login', { replace: true })
    })
    expect(replaceSpy).toHaveBeenCalledOnce()
    expect(window.location.pathname).toBe('/login')
    replaceSpy.mockRestore()
  })
})

describe('isActivePath', () => {
  it('matches exactly when end is set', () => {
    expect(isActivePath('/', '/', true)).toBe(true)
    expect(isActivePath('/calendar', '/', true)).toBe(false)
    expect(isActivePath('/calendar?date=x', '/calendar', true)).toBe(true)
  })

  it('matches the segment prefix otherwise', () => {
    expect(isActivePath('/calendar', '/calendar')).toBe(true)
    expect(isActivePath('/calendar', '/notes')).toBe(false)
    expect(isActivePath('/notes', '/calendar')).toBe(false)
  })

  it('ignores the query string when matching', () => {
    expect(isActivePath('/calendar?date=x', '/calendar')).toBe(true)
    expect(isActivePath('/calendar?date=x', '/notes')).toBe(false)
  })
})

describe('A', () => {
  function renderLink(props: Partial<React.ComponentProps<typeof A>> = {}) {
    return render(
      <>
        <A to="/tasks" {...props}>
          Tasks
        </A>
        <PathProbe />
      </>,
    )
  }

  it('renders a real anchor with the target href', () => {
    renderLink()
    expect(screen.getByRole('link', { name: 'Tasks' }).getAttribute('href')).toBe('/tasks')
  })

  it('navigates in-place on a plain click', () => {
    renderLink()
    fireEvent.click(screen.getByRole('link', { name: 'Tasks' }))
    expect(window.location.pathname).toBe('/tasks')
    expect(currentPath()).toBe('/tasks')
  })

  it('lets modified clicks through to the browser', () => {
    renderLink()
    for (const init of [{ metaKey: true }, { ctrlKey: true }, { shiftKey: true }, { altKey: true }]) {
      fireEvent.click(screen.getByRole('link', { name: 'Tasks' }), init)
      expect(window.location.pathname).toBe('/')
    }
    expect(currentPath()).toBe('/')
  })

  it('lets non-primary buttons through (middle-click opens a tab)', () => {
    renderLink()
    fireEvent.click(screen.getByRole('link', { name: 'Tasks' }), { button: 1 })
    expect(window.location.pathname).toBe('/')
    expect(currentPath()).toBe('/')
  })
})
