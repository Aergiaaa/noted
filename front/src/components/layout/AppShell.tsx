import { NavLink, Outlet } from 'react-router-dom'

const links = [
  { to: '/', label: 'Dashboard', end: true },
  { to: '/calendar', label: 'Calendar' },
  { to: '/tasks', label: 'Tasks' },
  { to: '/notes', label: 'Notes' },
  { to: '/finance', label: 'Finance' },
]

export default function AppShell() {
  return (
    <div className="min-h-screen bg-base-100 text-base-content">
      <nav className="navbar border-b border-base-300 bg-base-200 px-4">
        <div className="text-lg font-semibold">Noted</div>
        <div className="ml-6 flex gap-1">
          {links.map((l) => (
            <NavLink
              key={l.to}
              to={l.to}
              end={l.end}
              className={({ isActive }) =>
                `btn btn-ghost btn-sm ${isActive ? 'btn-active' : ''}`
              }
            >
              {l.label}
            </NavLink>
          ))}
        </div>
      </nav>
      <main className="mx-auto max-w-5xl p-4">
        <Outlet />
      </main>
    </div>
  )
}
