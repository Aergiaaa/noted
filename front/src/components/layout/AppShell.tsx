import { useState, type ReactNode } from 'react'
import { A, isActivePath, usePath } from '../../app/navigation'
import { useSession } from '../../lib/session'

const links = [
  { to: '/', label: 'Dashboard', end: true },
  { to: '/calendar', label: 'Calendar' },
  { to: '/tasks', label: 'Tasks' },
  { to: '/notes', label: 'Notes' },
  { to: '/finance', label: 'Finance' },
]

export default function AppShell({ children }: { children: ReactNode }) {
  const path = usePath()
  const { logout } = useSession()
  const [logoutFailed, setLogoutFailed] = useState(false)

  const onLogout = () => {
    setLogoutFailed(false)
    logout().catch(() => setLogoutFailed(true))
  }

  return (
    <div className="min-h-screen bg-base-100 text-base-content">
      <nav className="navbar border-b border-base-300 bg-base-200 px-4">
        <div className="navbar-start">
          <span className="text-lg font-semibold">Noted</span>
        </div>
        <div className="navbar-center gap-1">
          {links.map((l) => (
            <A
              key={l.to}
              to={l.to}
              className={`btn btn-ghost btn-sm ${isActivePath(path, l.to, l.end === true) ? 'btn-active' : ''}`}
            >
              {l.label}
            </A>
          ))}
        </div>
        <div className="navbar-end gap-2">
          {logoutFailed && (
            <span className="text-xs text-error" role="alert">
              Logout failed
            </span>
          )}
          <button className="btn btn-ghost btn-sm" onClick={onLogout}>
            Log out
          </button>
        </div>
      </nav>
      <main className="mx-auto max-w-5xl p-4">{children}</main>
    </div>
  )
}
