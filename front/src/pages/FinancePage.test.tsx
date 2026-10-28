import { afterEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import FinancePage from "./FinancePage";
import { toDayKey } from "../lib/format";
import { EVENTS_WIDE_PATH } from "../features/links/useLinkSources";
import type { BalanceResponse, Transaction, Wallet } from "../lib/api";
import { MemoryRouter } from "react-router";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function envelope(message: string, status: number) {
  return jsonResponse(status, { error: { code: "internal_error", message } });
}

const BALANCE: BalanceResponse = {
  currency: "IDR",
  total: { balance_cents: 1500000 },
  wallets: [{ wallet_id: "wal_cash", name: "Cash", balance_cents: 1500000 }],
};

const WALLETS: Wallet[] = [{ id: "wal_cash", name: "Cash", archived_at: null }];

function tx(over: Partial<Transaction> = {}): Transaction {
  return {
    id: "tx1",
    kind: "expense",
    amount_cents: 50000,
    currency: "IDR",
    occurred_on: "2026-10-20",
    memo: null,
    wallet_id: "wal_cash",
    from_wallet_id: null,
    to_wallet_id: null,
    task_id: null,
    event_id: null,
    note_id: null,
    ...over,
  };
}

// Route by URL+method; test-local branches never touch app coverage.
function stubFetch(
  route: (url: string, method: string) => Response | Promise<Response>,
) {
  const spy = vi.fn((input: RequestInfo | URL, init?: RequestInit) =>
    Promise.resolve(route(String(input), init?.method ?? "GET")),
  );
  vi.stubGlobal("fetch", spy);
  return spy;
}

function defaultRoute(
  txPage: { data: Transaction[]; next_cursor: string | null } = {
    data: [tx()],
    next_cursor: null,
  },
) {
  return (url: string, _method?: string): Response => {
    if (url === "/api/finance/balance") return jsonResponse(200, BALANCE);
    if (url === "/api/finance/wallets?archived=1") {
      return jsonResponse(200, { data: WALLETS });
    }
    if (url.startsWith("/api/finance/transactions")) {
      return jsonResponse(200, txPage);
    }
    throw new Error(`unexpected ${url}`);
  };
}

function calledUrls(spy: ReturnType<typeof vi.fn>): string[] {
  return spy.mock.calls.map((call) => String(call[0]));
}

describe("FinancePage", () => {
  it("shows a skeleton, then balance, strip, and history rows", async () => {
    let resolveFirst!: (r: Response) => void;
    let firstCall = true;
    const route = defaultRoute();
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      if (firstCall) {
        firstCall = false;
        return new Promise<Response>((resolve) => {
          resolveFirst = resolve;
        });
      }
      return Promise.resolve(route(String(input), init?.method ?? "GET"));
    });
    vi.stubGlobal("fetch", fetchMock);

    const { container } = render(<FinancePage />);
    expect(screen.getByRole("heading", { name: "Finance" })).toBeDefined();
    expect(container.querySelector(".skeleton")).not.toBeNull();

    await act(async () => {
      resolveFirst(jsonResponse(200, BALANCE));
    });

    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );
    expect(container.querySelector(".skeleton")).toBeNull();
    expect(screen.getByText("Rp 15,000")).toBeDefined();
    expect(screen.getAllByTestId("wallet-card")).toHaveLength(1);
    expect(calledUrls(fetchMock)).toEqual([
      "/api/finance/balance",
      "/api/finance/wallets?archived=1",
      "/api/finance/transactions",
      "/api/tasks",
      `/api${EVENTS_WIDE_PATH}`,
      "/api/notes",
    ]);
  });

  it("shows the load error and recovers on retry", async () => {
    let failBalance = true;
    const fetchMock = stubFetch((url) => {
      if (failBalance && url === "/api/finance/balance") {
        return envelope("boom", 500);
      }
      if (url === "/api/finance/balance") return jsonResponse(200, BALANCE);
      if (url === "/api/finance/wallets?archived=1") {
        return jsonResponse(200, { data: WALLETS });
      }
      return jsonResponse(200, { data: [tx()], next_cursor: null });
    });

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain("boom"),
    );

    failBalance = false;
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );
    expect(fetchMock).toHaveBeenCalled();
  });

  it("offers the create CTA when the history is empty", async () => {
    stubFetch((url) => {
      if (url === "/api/finance/balance") return jsonResponse(200, BALANCE);
      if (url === "/api/finance/wallets?archived=1") {
        return jsonResponse(200, { data: WALLETS });
      }
      return jsonResponse(200, { data: [], next_cursor: null });
    });

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getByText("No transactions yet.")).toBeDefined(),
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Record your first transaction" }),
    );
    expect(screen.getByLabelText("Amount")).toBeDefined();
  });

  it("toggles the history wallet filter from the strip", async () => {
    const fetchMock = stubFetch(defaultRoute());

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );

    const card = screen.getAllByTestId("wallet-card")[0];
    fireEvent.click(card);
    await waitFor(() =>
      expect(
        calledUrls(fetchMock).filter(
          (url) => url === "/api/finance/transactions?wallet_id=wal_cash",
        ),
      ).toHaveLength(1),
    );
    expect(card.getAttribute("aria-pressed")).toBe("true");

    fireEvent.click(card);
    await waitFor(() =>
      expect(
        calledUrls(fetchMock).filter(
          (url) => url === "/api/finance/transactions",
        ),
      ).toHaveLength(2),
    );
  });

  it("keeps balance and wallets when only the filter changes", async () => {
    const fetchMock = stubFetch(defaultRoute());

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );

    const card = screen.getAllByTestId("wallet-card")[0];
    fireEvent.click(card);
    fireEvent.click(card);
    await waitFor(() =>
      expect(
        calledUrls(fetchMock).filter((url) =>
          url.startsWith("/api/finance/transactions"),
        ),
      ).toHaveLength(3),
    );
    // The strip toggle only re-runs the history — balance and wallets stay
    // put (three transaction fetches: initial + filtered + unfiltered).
    expect(
      calledUrls(fetchMock).filter((url) => url === "/api/finance/balance"),
    ).toHaveLength(1);
    expect(
      calledUrls(fetchMock).filter(
        (url) => url === "/api/finance/wallets?archived=1",
      ),
    ).toHaveLength(1);
  });

  it("drops a stale response that lands after the filter changed", async () => {
    // Two stale rounds: one where the filtered page lands late (the success
    // handler must skip) and one where it fails late (the error handler must
    // skip). A slow response must never clobber the rows or surface an error
    // the user has already navigated away from.
    const staleTxs: {
      resolve: (r: Response) => void;
      reject: (reason: unknown) => void;
    }[] = [];
    let filteredCalls = 0;
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/finance/balance") {
        return Promise.resolve(jsonResponse(200, BALANCE));
      }
      if (url === "/api/finance/wallets?archived=1") {
        return Promise.resolve(jsonResponse(200, { data: WALLETS }));
      }
      if (url.includes("wallet_id=")) {
        filteredCalls += 1;
        return new Promise<Response>((resolve, reject) => {
          staleTxs.push({ resolve, reject });
        });
      }
      return Promise.resolve(
        jsonResponse(200, { data: [tx()], next_cursor: null }),
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );

    // Round 1 — a filtered success landing late must not replace the rows
    // (the stale page is deliberately wider: 2 rows would show a clobber).
    fireEvent.click(screen.getAllByTestId("wallet-card")[0]);
    await waitFor(() => expect(filteredCalls).toBe(1));
    fireEvent.click(screen.getAllByTestId("wallet-card")[0]);
    await waitFor(() =>
      expect(
        calledUrls(fetchMock).filter(
          (url) => url === "/api/finance/transactions",
        ),
      ).toHaveLength(2),
    );
    await act(async () => {
      staleTxs[0].resolve(
        jsonResponse(200, {
          data: [tx({ id: "s1" }), tx({ id: "s2" })],
          next_cursor: null,
        }),
      );
    });
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getAllByTestId("transaction-row")).toHaveLength(1);

    // Round 2 — a filtered failure landing late must not surface an error.
    fireEvent.click(screen.getAllByTestId("wallet-card")[0]);
    await waitFor(() => expect(filteredCalls).toBe(2));
    fireEvent.click(screen.getAllByTestId("wallet-card")[0]);
    await waitFor(() =>
      expect(
        calledUrls(fetchMock).filter(
          (url) => url === "/api/finance/transactions",
        ),
      ).toHaveLength(3),
    );
    await act(async () => {
      staleTxs[1].reject(new TypeError("stale"));
    });
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getAllByTestId("transaction-row")).toHaveLength(1);
  });

  it("drops a balance response that lands after a refetch replaced it", async () => {
    // A refetch (e.g. after void) can leave the balance in flight when the
    // next refetch replaces it; the stale settle (resolve and reject
    // rounds) must never touch state or surface an error.
    const pending: {
      resolve: (r: Response) => void;
      reject: (reason: unknown) => void;
    }[] = [];
    let balanceCalls = 0;
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if ((init?.method ?? "GET") === "DELETE") {
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      if (url === "/api/finance/balance") {
        balanceCalls += 1;
        if (balanceCalls === 2 || balanceCalls === 3) {
          return new Promise<Response>((resolve, reject) => {
            pending.push({ resolve, reject });
          });
        }
        return Promise.resolve(jsonResponse(200, BALANCE));
      }
      if (url === "/api/finance/wallets?archived=1") {
        return Promise.resolve(jsonResponse(200, { data: WALLETS }));
      }
      return Promise.resolve(
        jsonResponse(200, { data: [tx()], next_cursor: null }),
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );

    fireEvent.click(screen.getByRole("button", { name: "Void" }));
    fireEvent.click(screen.getByRole("button", { name: "Yes, void" }));
    await waitFor(() => expect(balanceCalls).toBe(2));

    // The row keeps its confirm state across refetches, so "Yes, void"
    // fires again while balance #2 is still in flight.
    fireEvent.click(screen.getByRole("button", { name: "Yes, void" }));
    await waitFor(() => expect(balanceCalls).toBe(3));

    await act(async () => {
      pending[0].resolve(jsonResponse(200, BALANCE));
    });
    expect(screen.queryByRole("alert")).toBeNull();

    // Third void; balance #3 goes stale on reject and must fail silently.
    fireEvent.click(screen.getByRole("button", { name: "Yes, void" }));
    await waitFor(() => expect(balanceCalls).toBe(4));

    await act(async () => {
      pending[1].reject(new TypeError("stale"));
    });
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getAllByTestId("transaction-row")).toHaveLength(1);
    expect(screen.getByText("Rp 15,000")).toBeDefined();
  });

  it("appends the next page through Load more", async () => {
    const fetchMock = stubFetch((url) => {
      if (url === "/api/finance/balance") return jsonResponse(200, BALANCE);
      if (url === "/api/finance/wallets?archived=1") {
        return jsonResponse(200, { data: WALLETS });
      }
      if (url === "/api/finance/transactions?cursor=c1") {
        return jsonResponse(200, {
          data: [tx({ id: "tx2" })],
          next_cursor: null,
        });
      }
      return jsonResponse(200, { data: [tx()], next_cursor: "c1" });
    });

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );

    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(2),
    );
    expect(
      calledUrls(fetchMock).filter((url) => url.includes("cursor=c1")),
    ).toHaveLength(1);
    expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  });

  it("surfaces the error when Load more fails", async () => {
    stubFetch((url) => {
      if (url === "/api/finance/balance") return jsonResponse(200, BALANCE);
      if (url === "/api/finance/wallets?archived=1") {
        return jsonResponse(200, { data: WALLETS });
      }
      if (url.includes("cursor=")) return envelope("gone", 500);
      return jsonResponse(200, { data: [tx()], next_cursor: "c1" });
    });

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Load more" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain("gone"),
    );
  });

  it("voids a transaction behind its confirm and refetches", async () => {
    const fetchMock = stubFetch((url, method) => {
      if (method === "DELETE" && url === "/api/finance/transactions/tx1") {
        return new Response(null, { status: 204 });
      }
      return defaultRoute()(url, method);
    });

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );

    fireEvent.click(screen.getByRole("button", { name: "Void" }));
    fireEvent.click(screen.getByRole("button", { name: "Yes, void" }));

    await waitFor(() =>
      expect(
        calledUrls(fetchMock).filter((url) => url === "/api/finance/balance"),
      ).toHaveLength(2),
    );
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/finance/transactions/tx1",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("shows the error when voiding fails", async () => {
    stubFetch((url, method) => {
      if (method === "DELETE") return envelope("nope", 500);
      return defaultRoute()(url, method);
    });

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );

    fireEvent.click(screen.getByRole("button", { name: "Void" }));
    fireEvent.click(screen.getByRole("button", { name: "Yes, void" }));
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain("nope"),
    );
  });

  it("records a transaction from the header and refetches", async () => {
    const fetchMock = stubFetch((url, method) => {
      if (method === "POST" && url === "/api/finance/transactions") {
        return jsonResponse(201, tx());
      }
      return defaultRoute()(url, method);
    });

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );

    fireEvent.click(screen.getByRole("button", { name: "Transaction" }));
    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "10" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    await waitFor(() => expect(screen.queryByLabelText("Amount")).toBeNull());
    const post = fetchMock.mock.calls.find(
      (call) =>
        String(call[0]) === "/api/finance/transactions" &&
        (call[1] as RequestInit | undefined)?.method === "POST",
    );
    expect(post?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({
        kind: "expense",
        wallet_id: "wal_cash",
        amount_cents: 1000000,
        occurred_on: toDayKey(new Date()),
      }),
    });
    // The refetch runs from the next effect, after the form's close
    // paint — count it there, not synchronously after waitFor.
    await waitFor(() =>
      expect(
        calledUrls(fetchMock).filter((url) => url === "/api/finance/balance"),
      ).toHaveLength(2),
    );
  });

  it("opens the transfer form and records a movement", async () => {
    const twoWalletBalance: BalanceResponse = {
      currency: "IDR",
      total: { balance_cents: 0 },
      wallets: [
        { wallet_id: "wal_cash", name: "Cash", balance_cents: 0 },
        { wallet_id: "wal_bank", name: "Bank", balance_cents: 0 },
      ],
    };
    const fetchMock = stubFetch((url, method) => {
      if (method === "POST" && url === "/api/finance/transfers") {
        return jsonResponse(201, {});
      }
      if (url === "/api/finance/balance") {
        return jsonResponse(200, twoWalletBalance);
      }
      return defaultRoute()(url, method);
    });

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );

    fireEvent.click(screen.getByRole("button", { name: "Transfer" }));
    expect(screen.getByLabelText("From")).toBeDefined();
    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "10" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Move" }));

    await waitFor(() => expect(screen.queryByLabelText("Amount")).toBeNull());
    const post = fetchMock.mock.calls.find(
      (call) =>
        String(call[0]) === "/api/finance/transfers" &&
        (call[1] as RequestInit | undefined)?.method === "POST",
    );
    expect(post?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({
        from_wallet_id: "wal_cash",
        to_wallet_id: "wal_bank",
        amount_cents: 1000000,
        occurred_on: toDayKey(new Date()),
      }),
    });
    await waitFor(() =>
      expect(
        calledUrls(fetchMock).filter((url) => url === "/api/finance/balance"),
      ).toHaveLength(2),
    );
  });

  it("opens the wallet manager and refetches after a change", async () => {
    const fetchMock = stubFetch((url, method) => {
      if (method === "PATCH" && url === "/api/finance/wallets/wal_cash") {
        return jsonResponse(200, {});
      }
      return defaultRoute()(url, method);
    });

    render(<FinancePage />);
    await waitFor(() =>
      expect(screen.getAllByTestId("transaction-row")).toHaveLength(1),
    );

    fireEvent.click(screen.getByRole("button", { name: "Manage wallets" }));
    expect(
      screen.getByRole("heading", { name: "Manage wallets" }),
    ).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "Archive" }));
    await waitFor(() =>
      expect(
        calledUrls(fetchMock).filter((url) => url === "/api/finance/balance"),
      ).toHaveLength(2),
    );

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(
      screen.queryByRole("heading", { name: "Manage wallets" }),
    ).toBeNull();
  });

  // Router for the chip tests: full history + the three title lists, and
  // a PATCH branch that clears the unlinked field on the next refetch
  // (each PATCH advances the modelled row) or errors without refetching.
  function chipRoute(patch: Response) {
    let patchCount = 0;
    return (url: string, method?: string): Response => {
      if (url === "/api/finance/balance") return jsonResponse(200, BALANCE);
      if (url === "/api/finance/wallets?archived=1") {
        return jsonResponse(200, { data: WALLETS });
      }
      if (url.startsWith("/api/finance/transactions")) {
        if (method === "PATCH") {
          patchCount += 1;
          return patch;
        }
        const row =
          patchCount === 0
            ? tx({ task_id: "t1", event_id: "e9", note_id: "n9" })
            : patchCount === 1
              ? tx({ event_id: "e9", note_id: "n9" })
              : tx({ event_id: "e9" });
        return jsonResponse(200, { data: [row], next_cursor: null });
      }
      if (url === "/api/tasks") {
        return jsonResponse(200, {
          data: [
            {
              id: "t1",
              title: "Buy keyboard",
              status: "open",
              due_date: null,
              event_id: null,
              note_id: null,
              completed_at: null,
            },
          ],
        });
      }
      if (url.startsWith("/api/events")) {
        return jsonResponse(200, {
          data: [{ id: "e9", title: "Dentist", starts_at: "", ends_at: "" }],
        });
      }
      if (url === "/api/notes") {
        return jsonResponse(200, {
          data: [{ id: "n9", title: "Groceries", body_md: "", pinned: false }],
        });
      }
      throw new Error(`unexpected ${url}`);
    };
  }

  it("titles the link chips and unlinks one behind its confirm", async () => {
    const fetchMock = stubFetch(chipRoute(new Response(null, { status: 204 })));

    render(
      <MemoryRouter>
        <FinancePage />
      </MemoryRouter>,
    );
    await waitFor(() =>
      expect(screen.getByRole("link", { name: "Buy keyboard" })).toBeDefined(),
    );
    const event = screen.getByRole("link", { name: "Dentist" });
    const note = screen.getByRole("link", { name: "Groceries" });
    expect(event.getAttribute("href")).toBe("/calendar?event=e9");
    expect(note.getAttribute("href")).toBe("/notes?id=n9");

    fireEvent.click(screen.getByRole("button", { name: "Unlink task" }));
    fireEvent.click(screen.getByRole("button", { name: "Unlink" }));

    await waitFor(() => {
      const patch = fetchMock.mock.calls.find(
        (call) => (call[1] as RequestInit | undefined)?.method === "PATCH",
      );
      expect(patch).toBeDefined();
      expect(String(patch![0])).toBe("/api/finance/transactions/tx1");
      expect(JSON.parse((patch![1] as { body: string }).body)).toEqual({
        task_id: "",
      });
    });
    await waitFor(() =>
      expect(screen.queryByRole("link", { name: "Buy keyboard" })).toBeNull(),
    );
    expect(screen.getByRole("link", { name: "Dentist" })).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "Unlink note" }));
    fireEvent.click(screen.getByRole("button", { name: "Unlink" }));

    await waitFor(() =>
      expect(screen.queryByRole("link", { name: "Groceries" })).toBeNull(),
    );
    const patches = fetchMock.mock.calls.filter(
      (call) => (call[1] as RequestInit | undefined)?.method === "PATCH",
    );
    expect(
      patches.map((patch) => JSON.parse((patch[1] as { body: string }).body)),
    ).toEqual([{ task_id: "" }, { note_id: "" }]);
    expect(screen.getByRole("link", { name: "Dentist" })).toBeDefined();
  });

  it("surfaces the error when unlinking fails", async () => {
    stubFetch(chipRoute(envelope("boom", 500)));

    render(
      <MemoryRouter>
        <FinancePage />
      </MemoryRouter>,
    );
    await waitFor(() =>
      expect(screen.getByRole("link", { name: "Buy keyboard" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Unlink event" }));
    fireEvent.click(screen.getByRole("button", { name: "Unlink" }));

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain("boom"),
    );
    expect(screen.getByRole("button", { name: "Retry" })).toBeDefined();
  });
});
