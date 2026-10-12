export interface ApiOptions {
  method?: string;
  body?: unknown;
}

// Plain data, not a class: thrown as-is and narrowed with isApiError
// (FRONTEND.md: no classes — Go struct + free functions).
export interface ApiError {
  readonly status: number;
  readonly code: string;
  readonly message: string;
  readonly fields?: Record<string, string>;
  readonly retryAfter?: number;
}

export function isApiError(err: unknown): err is ApiError {
  return typeof err === "object" && err !== null && "status" in err;
}

interface ErrorEnvelope {
  error?: { code?: string; message?: string; fields?: Record<string, string> };
}

// Single transport for every server call: JSON in/out, cookie included,
// errors thrown as ApiError so callers switch on status/code. No redirect
// here — lib/ stays router-free (FRONTEND.md mobile seam), the session
// guard owns redirect policy.
export async function api<T = unknown>(
  path: string,
  options: ApiOptions = {},
): Promise<T> {
  const { method = "GET", body } = options;
  const init: RequestInit = {
    method,
    credentials: "include",
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  };

  let res: Response;
  try {
    res = await fetch(`/api${path}`, init);
  } catch {
    throw {
      status: 0,
      code: "network_error",
      message: "network request failed",
    } satisfies ApiError;
  }

  if (res.ok) {
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  let code = "error";
  let message = "request failed";
  let fields: Record<string, string> | undefined;
  try {
    const envelope = (await res.json()) as ErrorEnvelope;
    if (envelope.error) {
      code = envelope.error.code ?? code;
      message = envelope.error.message ?? message;
      fields = envelope.error.fields;
    }
  } catch {
    // Non-JSON error body (proxy page, gateway) keeps the generic defaults.
  }

  const retryAfterHeader = res.headers.get("Retry-After");
  const retryAfter =
    retryAfterHeader === null ? undefined : Number(retryAfterHeader);
  throw {
    status: res.status,
    code,
    message,
    fields,
    retryAfter,
  } satisfies ApiError;
}
