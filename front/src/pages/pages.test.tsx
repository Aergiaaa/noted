import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import type {
  DashboardResponse,
  Event,
  Note,
  Task,
  Transaction,
} from "../lib/api";
import DashboardPage from "./DashboardPage";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function makeDashboardResponse(
  over: Partial<DashboardResponse> = {},
): DashboardResponse {
  const TODAY = "2026-10-29";
  const YESTERDAY = "2026-10-28";

  function event(over: Partial<Event> = {}): Event {
    return {
      id: "e1",
      title: "Team standup",
      starts_at: `${TODAY}T00:00:00Z`,
      ends_at: `${TODAY}T00:00:00Z`,
      description: null,
      note_id: null,
      ...over,
    };
  }

  function task(over: Partial<Task> = {}): Task {
    return {
      id: "t1",
      title: "Review PR",
      status: "open",
      due_date: TODAY,
      event_id: null,
      note_id: null,
      completed_at: null,
      ...over,
    };
  }

  function note(over: Partial<Note> = {}): Note {
    return {
      id: "n1",
      title: "Project ideas",
      body_md: "# Ideas\n- Item 1",
      pinned: true,
      ...over,
    };
  }

  function tx(over: Partial<Transaction> = {}): Transaction {
    return {
      id: "tx1",
      kind: "expense",
      amount_cents: 50000,
      currency: "IDR",
      occurred_on: YESTERDAY,
      memo: "Coffee",
      wallet_id: "wal_cash",
      from_wallet_id: null,
      to_wallet_id: null,
      task_id: null,
      event_id: null,
      note_id: null,
      ...over,
    };
  }

  return {
    date: TODAY,
    balance: {
      currency: "IDR",
      total: { balance_cents: 1500000 },
      wallets: [
        { wallet_id: "wal_cash", name: "Cash", balance_cents: 1500000 },
      ],
    },
    events_today: [event()],
    tasks_today: [task()],
    overdue_tasks: [
      task({ id: "t2", title: "Overdue task", due_date: YESTERDAY }),
    ],
    recent_transactions: [tx()],
    pinned_notes: [note()],
    ...over,
  };
}

// Shared fetch mock for dashboard page
let dashboardFetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  cleanup();
  if (dashboardFetchMock) {
    dashboardFetchMock.mockRestore();
  }
  dashboardFetchMock = vi
    .fn()
    .mockResolvedValue(jsonResponse(200, makeDashboardResponse()));
  vi.stubGlobal("fetch", dashboardFetchMock);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

// DashboardPage renders its heading with mocked API. Other pages have
// their own test files with proper mocks.
it("renders the Today at a glance heading", async () => {
  render(
    <MemoryRouter>
      <DashboardPage />
    </MemoryRouter>,
  );
  await waitFor(() =>
    expect(
      screen.getByRole("heading", { name: "Today at a glance" }),
    ).toBeDefined(),
  );
});
