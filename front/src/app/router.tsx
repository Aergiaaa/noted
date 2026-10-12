import { BrowserRouter, Navigate, Outlet, Route, Routes } from 'react-router'
import AppShell from '../components/layout/AppShell'
import ErrorState from '../components/ui/ErrorState'
import { useSession } from '../lib/session'
import CalendarPage from '../pages/CalendarPage'
import DashboardPage from '../pages/DashboardPage'
import FinancePage from '../pages/FinancePage'
import LoginPage from '../pages/LoginPage'
import NotesPage from '../pages/NotesPage'
import TasksPage from '../pages/TasksPage'

// The guard as structure: module routes hang off this layout route, so a
// guest never even matches them. Redirects replace so a guest's Back
// button does not bounce them into authed pages that no longer render.
function RequireSession() {
  const { status } = useSession()
  if (status !== 'authed') return <Navigate to="/login" replace />
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  )
}

// Declarative react-router (BrowserRouter + Routes; FRONTEND.md Routing).
// The session gate stays outside the router: no navigation may happen
// until we know who we are, so the URL is left untouched while loading
// or when the probe fails.
export function App() {
  const { status, retry } = useSession()

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

  return (
    <BrowserRouter>
      <Routes>
        <Route
          path="/login"
          element={status === 'guest' ? <LoginPage /> : <Navigate to="/" replace />}
        />
        <Route element={<RequireSession />}>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/calendar" element={<CalendarPage />} />
          <Route path="/tasks" element={<TasksPage />} />
          <Route path="/notes" element={<NotesPage />} />
          <Route path="/finance" element={<FinancePage />} />
        </Route>
        <Route path="*" element={<Navigate to={status === 'authed' ? '/' : '/login'} replace />} />
      </Routes>
    </BrowserRouter>
  )
}
