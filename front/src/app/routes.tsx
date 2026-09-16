import { Navigate, type RouteObject } from 'react-router-dom'
import AppShell from '../components/layout/AppShell'
import CalendarPage from '../pages/CalendarPage'
import DashboardPage from '../pages/DashboardPage'
import FinancePage from '../pages/FinancePage'
import LoginPage from '../pages/LoginPage'
import NotesPage from '../pages/NotesPage'
import TasksPage from '../pages/TasksPage'

// F1 skeleton: placeholder routes, no auth guard (guard lands in F4/F5).
// Unknown route → `/`. Exported separately so tests can assert the
// table without creating browser history (needs DOM).
export const routes: RouteObject[] = [
  { path: '/login', element: <LoginPage /> },
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <DashboardPage /> },
      { path: 'calendar', element: <CalendarPage /> },
      { path: 'tasks', element: <TasksPage /> },
      { path: 'notes', element: <NotesPage /> },
      { path: 'finance', element: <FinancePage /> },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
]
