import { useEffect, useState, type MouseEvent, type ReactNode } from 'react'

// The whole router lives on the platform History API (FRONTEND.md
// Routing): pushState notifies our own listeners, the browser notifies
// us on back/forward via popstate.
const NAVIGATE_EVENT = 'noted:navigate'

export function navigate(to: string, options: { replace?: boolean } = {}): void {
  if (options.replace === true) {
    window.history.replaceState(null, '', to)
  } else {
    window.history.pushState(null, '', to)
  }
  window.dispatchEvent(new Event(NAVIGATE_EVENT))
}

export function usePath(): string {
  const [path, setPath] = useState(() => window.location.pathname + window.location.search)

  useEffect(() => {
    const sync = () => setPath(window.location.pathname + window.location.search)
    window.addEventListener('popstate', sync)
    window.addEventListener(NAVIGATE_EVENT, sync)
    return () => {
      window.removeEventListener('popstate', sync)
      window.removeEventListener(NAVIGATE_EVENT, sync)
    }
  }, [])

  return path
}

function pathnameOf(path: string): string {
  const query = path.indexOf('?')
  return query === -1 ? path : path.slice(0, query)
}

export function isActivePath(path: string, to: string, end = false): boolean {
  const pathname = pathnameOf(path)
  if (end) return pathname === to
  return pathname === to || pathname.startsWith(`${to}/`)
}

interface AProps {
  to: string
  className?: string
  children: ReactNode
}

// Anchors, not buttons: cmd/ctrl/middle-click must still open a new tab,
// which only a real href gives the browser. Active styling is the caller's
// business (isActivePath) — links are not all buttons.
export function A({ to, className, children }: AProps) {
  const onClick = (event: MouseEvent<HTMLAnchorElement>) => {
    const modified = event.metaKey || event.ctrlKey || event.shiftKey || event.altKey
    if (modified || event.button !== 0) return
    event.preventDefault()
    navigate(to)
  }

  return (
    <a href={to} onClick={onClick} className={className}>
      {children}
    </a>
  )
}
