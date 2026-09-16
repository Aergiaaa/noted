import { describe, expect, it } from 'vitest'
import type { RouteObject } from 'react-router-dom'
import { routes } from './routes'

function flatten(rs: RouteObject[], base = ''): string[] {
  return rs.flatMap((r) => {
    if (r.index) return [`${base}/`]
    const full = `${base}/${r.path ?? ''}`.replace(/\/+/g, '/')
    const kids = r.children ? flatten(r.children, full === '/' ? '' : full) : []
    return [full, ...kids]
  })
}

describe('skeleton routes', () => {
  it('exposes every F1 placeholder route', () => {
    const paths = flatten(routes)
    for (const p of ['/login', '/', '/calendar', '/tasks', '/notes', '/finance']) {
      expect(paths).toContain(p)
    }
  })

  it('redirects unknown routes to /', () => {
    const fallback = routes.find((r) => r.path === '*')
    expect(fallback).toBeDefined()
    // Navigate element targets "/"
    const el = fallback?.element as unknown as { props: { to: string } }
    expect(el.props.to).toBe('/')
  })

  it('uses replace so the bad URL does not stay in history', () => {
    const fallback = routes.find((r) => r.path === '*')
    const el = fallback?.element as unknown as { props: { replace: boolean } }
    expect(el.props.replace).toBe(true)
  })

  it('keeps /login outside the authenticated shell', () => {
    const login = routes.find((r) => r.path === '/login')
    expect(login).toBeDefined()
    expect(login?.children).toBeUndefined()
  })

  it('nests exactly the five module pages under the shell', () => {
    const shell = routes.find((r) => r.path === '/')
    const kids = shell?.children ?? []
    // index (dashboard) + calendar + tasks + notes + finance
    expect(kids).toHaveLength(6 - 1)
    expect(kids.filter((k) => k.index)).toHaveLength(1)
    const paths = kids.filter((k) => !k.index).map((k) => k.path)
    expect(paths.sort()).toEqual(['calendar', 'finance', 'notes', 'tasks'])
  })
})
