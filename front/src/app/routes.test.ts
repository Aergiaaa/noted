import { describe, expect, it } from 'vitest'
import { findRoute, routes } from './routes'

describe('routes', () => {
  it('exposes the five module paths plus /login', () => {
    expect(routes.map((r) => r.path).sort()).toEqual([
      '/',
      '/calendar',
      '/finance',
      '/login',
      '/notes',
      '/tasks',
    ])
  })

  it('returns undefined for unknown paths so the guard can redirect', () => {
    expect(findRoute('/missing')).toBeUndefined()
  })

  it('finds each declared path', () => {
    for (const route of routes) {
      expect(findRoute(route.path)).toBe(route)
    }
  })
})
