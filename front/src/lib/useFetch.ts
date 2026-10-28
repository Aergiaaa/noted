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
// skeleton back on. An AbortController cancels the previous fetch when the
// effect re-runs or unmounts, so a stale response never overwrites a newer
// one. Aborted requests are silent — they don't set error or flip loading,
// because the replacement fetch owns that state. Loading clears when any
// non-aborted fetch settles (success or failure); the page decides what to
// render (ErrorState only when data is null, grid otherwise — keep-previous).
// A fetch hook is orchestration, which PLAYBOOK §4 allows — only trivial
// boolean hooks are banned.
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
    const controller = new AbortController();

    api<T>(path, { signal: controller.signal })
      .then((res) => {
        setData(res);
        setError(null);
        setLoading(false);
      })

      .catch((err: unknown) => {
        if (controller.signal.aborted) {
          return;
        }

        setError(isApiError(err) ? err.message : fallbackError);
        setLoading(false);
      });

    return () => {
      controller.abort();
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
