import { useEffect, useState, type SubmitEvent } from "react";
import {
  isApiError,
  STATUS_UNAUTHENTICATED,
  STATUS_TOO_MANY_REQUESTS,
} from "../lib/api";
import { useSession } from "../lib/session";

// api() throws ApiError data for every HTTP/network failure; anything else
// (a leaked body-parse error) falls through to the generic message.
function loginErrorMessage(err: unknown): string {
  if (!isApiError(err)) return "Login failed. Try again.";
  if (err.status === STATUS_UNAUTHENTICATED) return "Invalid code. Try again.";
  if (err.status === STATUS_TOO_MANY_REQUESTS)
    return "Too many attempts. Try again later.";
  if (err.status === 0)
    return "Could not reach the server. Check the connection and retry.";
  return "Login failed. Try again.";
}

// Seconds to lock the form after a 429, or null when there is no usable
// Retry-After (missing, zero, or non-numeric) — the backend's 5/min/IP
// bucket keeps rejecting attempts either way.
function lockoutSeconds(err: unknown): number | null {
  if (!isApiError(err) || err.status !== STATUS_TOO_MANY_REQUESTS) return null;
  const retryAfter = err.retryAfter;
  return typeof retryAfter === "number" &&
    Number.isFinite(retryAfter) &&
    retryAfter > 0
    ? retryAfter
    : null;
}

const CODE_REQUIRED = "Enter the 6-digit code or a recovery code.";
const RATE_LIMITED = "Too many attempts. Try again in";

export default function LoginPage() {
  const { login } = useSession();
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);
  const [lockedUntil, setLockedUntil] = useState<number | null>(null);
  const [secondsLeft, setSecondsLeft] = useState(0);

  const onSubmit = (event: SubmitEvent) => {
    event.preventDefault();
    if (checking || lockedUntil !== null) return;
    const attempted = code.trim();
    if (attempted === "") {
      setError(CODE_REQUIRED);
      return;
    }
    setCode("");
    setChecking(true);
    setError(null);
    login(attempted)
      .catch((err: unknown) => {
        const lockout = lockoutSeconds(err);
        if (lockout !== null) {
          setLockedUntil(Date.now() + lockout * 1000);
          setSecondsLeft(lockout);
          return;
        }
        setError(loginErrorMessage(err));
      })
      .finally(() => setChecking(false));
  };

  // Wall-clock deadline, not a decrementing counter: a throttled
  // background tab can pause ticks, and the next one self-corrects.
  useEffect(() => {
    if (lockedUntil === null) return;
    const unlock = () => {
      setLockedUntil(null);
      setSecondsLeft(0);
    };
    const id = setInterval(() => {
      const left = Math.ceil((lockedUntil - Date.now()) / 1000);
      if (left <= 0) unlock();
      else setSecondsLeft(left);
    }, 1000);
    return () => clearInterval(id);
  }, [lockedUntil]);

  const locked = lockedUntil !== null;
  const alert =
    error ?? (secondsLeft > 0 ? `${RATE_LIMITED} ${secondsLeft}s.` : null);

  return (
    <section className="flex min-h-screen items-center justify-center p-4">
      <form
        className="card w-full max-w-sm bg-base-200 border border-base-300 p-4"
        onSubmit={onSubmit}
      >
        <h1 className="text-xl font-semibold">Login</h1>
        <p className="mt-1 text-sm text-base-content/70">
          Enter the 6-digit code from your authenticator app, or a recovery
          code.
        </p>
        <label className="form-control mt-3">
          <span className="label-text text-xs uppercase text-base-content/60">
            Code
          </span>
          <input
            className="input input-bordered input-sm w-full"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            autoComplete="one-time-code"
            inputMode="text"
            aria-label="Code"
            disabled={locked}
          />
        </label>
        {/* Fixed one-line slot (text-xs at leading 1.5 = 18px) so the card
            stays put between messages; also keeps the live region mounted
            for screen readers. Long messages may still wrap and grow. */}
        <p className="mt-2 min-h-4.5 text-xs text-error" role="alert">
          {alert ?? ""}
        </p>
        <button
          type="submit"
          className={`btn btn-primary btn-sm mt-3 ${checking || locked ? "btn-disabled" : ""}`}
          disabled={checking || locked}
        >
          {checking ? "Checking…" : "Log in"}
        </button>
      </form>
    </section>
  );
}
