import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { api, ApiError } from './api'

export type SessionStatus = 'loading' | 'authed' | 'guest' | 'error'

interface SessionValue {
  status: SessionStatus
  login: (code: string) => Promise<void>
  logout: () => Promise<void>
  retry: () => void
}

const SessionContext = createContext<SessionValue | null>(null)

export function useSession(): SessionValue {
  const value = useContext(SessionContext)
  if (value === null) throw new Error('useSession must be used within SessionProvider')
  return value
}

// One probe per mount decides the initial status; retry re-runs it.
// Only 401 means "guest" — a network failure must reach the guard as
// "error" so the user gets a retry instead of a login wall.
export function SessionProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<SessionStatus>('loading')
  const [probe, setProbe] = useState(0)

  useEffect(() => {
    let cancelled = false
    api<{ authenticated: boolean }>('/session')
      .then(() => {
        if (!cancelled) setStatus('authed')
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setStatus(err instanceof ApiError && err.status === 401 ? 'guest' : 'error')
      })
    return () => {
      cancelled = true
    }
  }, [probe])

  const login = useCallback(async (code: string) => {
    await api('/auth/login', { method: 'POST', body: { code } })
    setStatus('authed')
  }, [])

  const logout = useCallback(async () => {
    await api('/auth/logout', { method: 'POST' })
    setStatus('guest')
  }, [])

  const retry = useCallback(() => {
    setStatus('loading')
    setProbe((p) => p + 1)
  }, [])

  const value = useMemo(
    () => ({ status, login, logout, retry }),
    [status, login, logout, retry],
  )

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}
