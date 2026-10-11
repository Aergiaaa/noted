import { useEffect } from 'react'
import AppShell from '../components/layout/AppShell'
import ErrorState from '../components/ui/ErrorState'
import { useSession } from '../lib/session'
import { navigate, usePath } from './navigation'
import { findRoute } from './routes'

// The guard and the switch, hand-rolled on the History API
// (FRONTEND.md Routing). Redirects replace so a guest's Back button
// does not bounce them into authed pages that no longer render.
export function App() {
  const path = usePath()
  const { status, retry } = useSession()
  const pathname = path.split('?')[0]
  const route = findRoute(pathname)

  useEffect(() => {
    if (status === 'loading' || status === 'error') return
    if (status === 'guest' && pathname !== '/login') {
      navigate('/login', { replace: true })
    }
    if (status === 'authed' && (pathname === '/login' || route === undefined)) {
      navigate('/', { replace: true })
    }
  }, [status, pathname, route])

  if (status === 'loading') {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <span className="loading loading-spinner loading-lg" role="status" aria-label="Loading" />
      </div>
    )
  }

  if (status === 'error') {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <ErrorState message="Could not reach the server" onRetry={retry} />
      </div>
    )
  }

  // Redirects run in the effect above; render nothing until then so a
  // guest never sees the authed page that is about to unmount.
  if (status === 'guest') {
    // Only /login survives the guard for guests.
    if (pathname === '/login' && route !== undefined) return <div>{route.element}</div>
    return null
  }

  // authed: /login or unknown is mid-redirect; everything else is a
  // module page inside the shell.
  if (pathname === '/login' || route === undefined) return null
  return <AppShell>{route.element}</AppShell>
}
