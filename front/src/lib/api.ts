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

// Events are UTC instants; list endpoints answer {data} (API.md).
export interface Event {
  id: string;
  title: string;
  starts_at: string;
  ends_at: string;
  description?: string | null;
  note_id?: string | null;
}

export interface EventsResponse {
  data: Event[];
}

// Notes are plain markdown documents; tasks are open/done actions with
// optional calendar-day due dates and module links (API.md list rows).
export interface Note {
  id: string;
  title: string;
  body_md: string;
  pinned: boolean;
}

export interface Task {
  id: string;
  title: string;
  status: "open" | "done";
  due_date: string | null;
  event_id: string | null;
  note_id: string | null;
  completed_at: string | null;
}

export interface NotesResponse {
  data: Note[];
}

export interface TasksResponse {
  data: Task[];
}

// Finance (API.md): wallets archive, never delete; money moves as
// income/expense (wallet_id) or a single transfer row (from/to sides).
// wal_cash is the default pot seeded by migration 0005 (DATABASE.md) —
// part of the schema contract, not an arbitrary client choice.
export const WALLET_SEED_CASH_ID = "wal_cash";

export interface Wallet {
  id: string;
  name: string;
  archived_at: string | null;
}

export interface WalletsResponse {
  data: Wallet[];
}

export interface WalletBalance {
  wallet_id: string;
  name: string;
  balance_cents: number;
}

export interface BalanceResponse {
  currency: string;
  total: { balance_cents: number };
  wallets: WalletBalance[];
}

export type TransactionKind = "income" | "expense" | "transfer";

export interface Transaction {
  id: string;
  kind: TransactionKind;
  amount_cents: number;
  currency: string;
  occurred_on: string;
  memo: string | null;
  wallet_id: string | null;
  from_wallet_id: string | null;
  to_wallet_id: string | null;
  task_id: string | null;
  event_id: string | null;
  note_id: string | null;
}

export interface TransactionsResponse {
  data: Transaction[];
  next_cursor: string | null;
}

// HTTP status codes we branch on (API.md envelope contract).
export const STATUS_UNAUTHENTICATED = 401;
export const STATUS_TOO_MANY_REQUESTS = 429;

// HTTP 204 No Content: success with no body to parse (delete/logout per API.md).
const STATUS_NO_CONTENT = 204;

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
    if (res.status === STATUS_NO_CONTENT) return undefined as T;
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

// Server envelope fields (API.md validation_error) become "field: message";
// anything else falls back to the caller's generic failure line.
export function errorLines(err: unknown, fallback: string): string[] {
  if (!isApiError(err)) return [fallback];
  if (err.fields !== undefined) {
    return Object.entries(err.fields).map(
      ([field, message]) => `${field}: ${message}`,
    );
  }
  return [err.message];
}
