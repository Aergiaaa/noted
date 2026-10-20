import { useCallback, useEffect, useState } from "react";
import { api, isApiError } from "./api";

export interface FetchState<T> {
  data: T | null;
  error: string | null;
  loading: boolean;
  refetch: () => void;
  retry: () => void;
}

// Shared GET lifecycle: loading starts true, data/errors land from the
// latest fetch, path changes clear the error but keep previous data (the
// grid must stay mounted so the week-slide animation can play behind the
// swap — no skeleton flash), refetch re-runs silently, retry flips the
// skeleton back on. Loading only ever clears in the non-cancelled finally:
// a stale (cancelled) request never touches the flag the replacing fetch
// owns, and navigation never re-shows the skeleton. A fetch hook is
// orchestration, which PLAYBOOK §4 allows — only trivial boolean hooks are
// banned.
export function useFetch<T>(
  path: string,
  fallbackError: string,
): FetchState<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [nonce, setNonce] = useState(0);
  const [prevPath, setPrevPath] = useState(path);

  // Render-phase reset (compare, don't set-in-effect): a URL change means
  // a new resource, so the old error is stale; keep data for keep-previous.
  if (prevPath !== path) {
    setPrevPath(path);
    setError(null);
  }

  useEffect(() => {
    let cancelled = false;

    api<T>(path)
      .then((res) => {
        if (!cancelled) {
          setData(res);
          setError(null);
        }
      })

      .catch((err: unknown) => {
        if (!cancelled) setError(isApiError(err) ? err.message : fallbackError);
      })

      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [path, nonce, fallbackError]);

  const refetch = useCallback(() => {
    setError(null);
    setNonce((n) => n + 1);
  }, []);

  const retry = useCallback(() => {
    setError(null);
    setLoading(true);
    setNonce((n) => n + 1);
  }, []);

  return { data, error, loading, refetch, retry };
}
