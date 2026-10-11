import type { ReactNode } from 'react'
import CalendarPage from '../pages/CalendarPage'
import DashboardPage from '../pages/DashboardPage'
import FinancePage from '../pages/FinancePage'
import LoginPage from '../pages/LoginPage'
import NotesPage from '../pages/NotesPage'
import TasksPage from '../pages/TasksPage'

export interface AppRoute {
  path: string
  element: ReactNode
}

// Flat table, hand-matched (FRONTEND.md Routing). /login is the one
// non-shell page; the guard (router.tsx) keeps guests there and authed
// users off it, so no shell flag is needed.
export const routes: AppRoute[] = [
  { path: '/login', element: <LoginPage /> },
  { path: '/', element: <DashboardPage /> },
  { path: '/calendar', element: <CalendarPage /> },
  { path: '/tasks', element: <TasksPage /> },
  { path: '/notes', element: <NotesPage /> },
  { path: '/finance', element: <FinancePage /> },
]

export function findRoute(pathname: string): AppRoute | undefined {
  return routes.find((route) => route.path === pathname)
}
