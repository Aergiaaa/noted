import { afterEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { App } from "./router";
import { SessionProvider } from "../lib/session";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
});

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function stubProbe(fetchMock: ReturnType<typeof vi.fn>) {
  vi.stubGlobal("fetch", fetchMock);
}

function stubAuthed(extra: Array<[string, Response]> = []) {
  // Dashboard response for authenticated users who land on /
  const dashboardResponse = jsonResponse(200, {
    date: "2026-10-29",
    balance: { currency: "USD", total: { balance_cents: 0 }, wallets: [] },
    events_today: [],
    tasks_today: [],
    overdue_tasks: [],
    recent_transactions: [],
    pinned_notes: [],
  });

  // Events list response for Calendar page
  const eventsResponse = jsonResponse(200, { data: [] });

  // Transactions list response for Finance page
  const transactionsResponse = jsonResponse(200, {
    data: [],
    next_cursor: null,
  });

  // Wallets response for Finance page
  const walletsResponse = jsonResponse(200, { data: [] });

  // Notes list response for Notes page
  const notesResponse = jsonResponse(200, { data: [] });

  // Tasks list response for Tasks page
  const tasksResponse = jsonResponse(200, { data: [] });

  const fetchMock = vi.fn((input: RequestInfo | URL, _init?: RequestInit) => {
    const url = String(input);
    // Session probe
    if (url === "/api/session") {
      return Promise.resolve(
        jsonResponse(200, { authenticated: true, expires_at: "x" }),
      );
    }
    // Dashboard (api() prepends /api)
    if (url.startsWith("/api/dashboard")) {
      return Promise.resolve(dashboardResponse);
    }
    // Events
    if (url.startsWith("/api/events")) {
      return Promise.resolve(eventsResponse);
    }
    // Finance
    if (url.startsWith("/api/finance/transactions")) {
      return Promise.resolve(transactionsResponse);
    }
    if (url.startsWith("/api/finance/wallets")) {
      return Promise.resolve(walletsResponse);
    }
    // Notes
    if (url.startsWith("/api/notes")) {
      return Promise.resolve(notesResponse);
    }
    // Tasks
    if (url.startsWith("/api/tasks")) {
      return Promise.resolve(tasksResponse);
    }
    return Promise.resolve(
      jsonResponse(404, { error: { code: "not_found", message: "not found" } }),
    );
  });

  for (const [url, res] of extra) {
    // Allow extra responses to override specific URLs
    const originalImpl = fetchMock.getMockImplementation();
    fetchMock.mockImplementation(
      (input: RequestInfo | URL, init?: RequestInit) => {
        const u = String(input);
        if (u === url) return Promise.resolve(res);
        return (
          originalImpl?.(input, init) ??
          Promise.resolve(
            jsonResponse(404, {
              error: { code: "not_found", message: "not found" },
            }),
          )
        );
      },
    );
  }
  stubProbe(fetchMock);
  return fetchMock;
}

function stubGuest() {
  stubProbe(
    vi.fn().mockResolvedValue(
      jsonResponse(401, {
        error: { code: "unauthenticated", message: "nope" },
      }),
    ),
  );
}

function renderApp() {
  return render(
    <SessionProvider>
      <App />
    </SessionProvider>,
  );
}

describe("App guard", () => {
  it("shows a skeleton while the session probe is in flight", () => {
    // A never-resolving fetch keeps status at "loading".
    vi.stubGlobal(
      "fetch",
      vi.fn().mockReturnValue(new Promise<Response>(() => {})),
    );

    renderApp();

    expect(screen.getByRole("status", { name: "Loading" })).toBeDefined();
  });

  it("offers retry when the probe fails, then proceeds", async () => {
    const dashboardResponse = jsonResponse(200, {
      date: "2026-10-29",
      balance: { currency: "USD", total: { balance_cents: 0 }, wallets: [] },
      events_today: [],
      tasks_today: [],
      overdue_tasks: [],
      recent_transactions: [],
      pinned_notes: [],
    });

    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("offline"))
      .mockResolvedValueOnce(
        jsonResponse(200, { authenticated: true, expires_at: "x" }),
      )
      .mockResolvedValue(dashboardResponse);
    stubProbe(fetchMock);

    renderApp();
    await waitFor(() => expect(screen.getByRole("alert")).toBeDefined());

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    });

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );
    expect(fetchMock).toHaveBeenCalledTimes(3); // session retry + dashboard
  });

  it("sends guests to /login, replacing the history entry", async () => {
    window.history.replaceState(null, "", "/tasks");
    stubGuest();

    renderApp();

    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "Login" })).toBeDefined(),
    );
    expect(window.location.pathname).toBe("/login");
  });

  it("sends guests from unknown paths to /login", async () => {
    window.history.replaceState(null, "", "/missing-page");
    stubGuest();

    renderApp();

    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "Login" })).toBeDefined(),
    );
    expect(window.location.pathname).toBe("/login");
  });

  it("lets guests sit on /login", async () => {
    window.history.replaceState(null, "", "/login");
    stubGuest();

    renderApp();

    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "Login" })).toBeDefined(),
    );
    expect(window.location.pathname).toBe("/login");
  });

  it("wraps module routes in the shell for authed users", async () => {
    window.history.replaceState(null, "", "/calendar");
    stubAuthed();

    renderApp();

    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "Calendar" })).toBeDefined(),
    );
    expect(screen.getByText("Noted")).toBeDefined();
    expect(window.location.pathname).toBe("/calendar");
  });

  it("redirects authed users off /login to the dashboard", async () => {
    window.history.replaceState(null, "", "/login");
    stubAuthed();

    renderApp();

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );
    expect(window.location.pathname).toBe("/");
  });

  it("redirects authed users from unknown paths to /", async () => {
    window.history.replaceState(null, "", "/missing-page");
    stubAuthed();

    renderApp();

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );
    expect(window.location.pathname).toBe("/");
  });

  it("renders /login outside the shell", async () => {
    window.history.replaceState(null, "", "/login");
    stubGuest();

    renderApp();

    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "Login" })).toBeDefined(),
    );
    expect(screen.queryByText("Noted")).toBeNull();
  });
});
