import { useState, type FormEvent } from 'react'
import { ApiError } from '../lib/api'
import { useSession } from '../lib/session'

// api() rejects only with ApiError, so no instanceof fallback is needed.
function loginErrorMessage(err: ApiError): string {
  if (err.status === 401) return 'Invalid code. Try again.'
  if (err.status === 429) {
    return err.retryAfter === undefined
      ? 'Too many attempts. Try again later.'
      : `Too many attempts. Try again in ${err.retryAfter}s.`
  }
  if (err.status === 0) return 'Could not reach the server. Check the connection and retry.'
  return 'Login failed. Try again.'
}

const CODE_REQUIRED = 'Enter the 6-digit code or a recovery code.'

export default function LoginPage() {
  const { login } = useSession()
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [checking, setChecking] = useState(false)

  const onSubmit = (event: FormEvent) => {
    event.preventDefault()
    if (code.trim() === '') {
      setError(CODE_REQUIRED)
      return
    }
    setChecking(true)
    setError(null)
    login(code.trim())
      .catch((err: ApiError) => setError(loginErrorMessage(err)))
      .finally(() => setChecking(false))
  }

  return (
    <section className="mx-auto mt-16 max-w-sm">
      <form className="card bg-base-200 border border-base-300 p-4" onSubmit={onSubmit}>
        <h1 className="text-xl font-semibold">Login</h1>
        <p className="mt-1 text-sm text-base-content/70">
          Enter the 6-digit code from your authenticator app, or a recovery code.
        </p>
        <label className="form-control mt-3">
          <span className="label-text text-xs uppercase text-base-content/60">Code</span>
          <input
            className="input input-bordered input-sm w-full"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            autoComplete="one-time-code"
            inputMode="text"
            aria-label="Code"
          />
        </label>
        {error !== null && (
          <p className="mt-2 text-xs text-error" role="alert">
            {error}
          </p>
        )}
        <button
          type="submit"
          className={`btn btn-primary btn-sm mt-3 ${checking ? 'btn-disabled' : ''}`}
          disabled={checking}
        >
          {checking ? 'Checking…' : 'Log in'}
        </button>
      </form>
    </section>
  )
}
