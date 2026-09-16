import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import CalendarPage from './CalendarPage'
import DashboardPage from './DashboardPage'
import FinancePage from './FinancePage'
import LoginPage from './LoginPage'
import NotesPage from './NotesPage'
import TasksPage from './TasksPage'

afterEach(() => {
  cleanup()
})

// Every F1 placeholder page renders its heading. If a page stops
// rendering (typo, bad export), the router tests below would show a blank
// outlet — these pin each page in isolation.
describe.each([
  ['dashboard', <DashboardPage />, 'Dashboard'],
  ['calendar', <CalendarPage />, 'Calendar'],
  ['tasks', <TasksPage />, 'Tasks'],
  ['notes', <NotesPage />, 'Notes'],
  ['finance', <FinancePage />, 'Finance'],
  ['login', <LoginPage />, 'Login'],
] as const)('%s page', (_name, element, heading) => {
  it(`renders the ${heading} heading`, () => {
    render(element)
    expect(screen.getByRole('heading', { name: heading })).toBeDefined()
  })
})
