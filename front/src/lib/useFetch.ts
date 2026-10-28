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
// one (the old race the docs called out). Aborted requests are silent —
// they don't set error or flip loading, because the replacement fetch owns
// that state. A fetch hook is orchestration, which PLAYBOOK §4 allows —
// only trivial boolean hooks are banned.
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
    console.log("[useFetch] effect running, nonce:", nonce);
    const controller = new AbortController();
    const isBackground = nonce > 0;
    let fetchError: string | null = null;

    console.log("[useFetch] calling api, nonce:", nonce);
    api<T>(path, { signal: controller.signal })
      .then((res) => {
        // Check if response is an ApiError (HTTP error from server)
        if (isApiError(res)) {
          console.log("[useFetch] HTTP error:", res);
          fetchError = res.message;
          setError(res.message);
          // On HTTP error: initial load clears loading; background sets loading=true
          if (isBackground) {
            setLoading(true);
          } else {
            setLoading(false);
          }
          return;
        }
        // Success
        setData(res);
        setError(null);
      })

      .catch((err: unknown) => {
        // Network error (fetch threw) - wrap as ApiError with status=0
        console.log("[useFetch] network error:", err);
        if (err instanceof DOMException && err.name === "AbortError") {
          return;
        }
        fetchError = isApiError(err) ? err.message : fallbackError;
        setError(fetchError);
        // On network error: initial load clears loading; background sets loading=true
        if (isBackground) {
          setLoading(true);
        } else {
          setLoading(false);
        }
      })

      .then(() => {
        // On success (no error): clear loading for non-aborted fetches
        if (!controller.signal.aborted && fetchError === null) {
          setLoading(false);
        }
      })

      .finally(() => {
        // Aborted fetches don't own the loading flag — the replacement owns it.
        // Non-aborted: loading already set correctly.
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
