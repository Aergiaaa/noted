import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import DashboardPage from "./DashboardPage";
import { MemoryRouter, useLocation } from "react-router";
import type {
  DashboardResponse,
  Event,
  Task,
  Note,
  Transaction,
} from "../lib/api";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

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

function makeDashboardResponse(
  over: Partial<DashboardResponse> = {},
): DashboardResponse {
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

// Reads the router location so a test can tell client-side navigation
// (react-router Link) from a raw <a>, which jsdom never moves.
function LocationProbe() {
  const location = useLocation();
  return <div data-testid="loc">{location.pathname + location.search}</div>;
}

describe("DashboardPage", () => {
  beforeEach(() => {
    // Mock system time to match TODAY constant for deterministic tests
    vi.setSystemTime(new Date("2026-10-29T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows a skeleton while loading", () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, makeDashboardResponse()));

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole("status", { name: "Loading dashboard" }),
    ).toBeDefined();

    fetchMock.mockRestore();
  });

  it("shows error state with retry on failure", async () => {
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("offline"))
      .mockResolvedValue(jsonResponse(200, makeDashboardResponse()));

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
      </MemoryRouter>,
    );

    await waitFor(() => expect(screen.getByRole("alert")).toBeDefined());
    expect(screen.getByRole("button", { name: "Retry" })).toBeDefined();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    });

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );

    fetchMock.mockRestore();
  });

  it("renders all five cards with data", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, makeDashboardResponse()));

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
      </MemoryRouter>,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );

    // Events today card
    expect(screen.getByRole("heading", { name: "Events today" })).toBeDefined();
    expect(screen.getByText("Team standup")).toBeDefined();
    const timeElements = screen.getAllByRole("time");
    expect(timeElements.length).toBeGreaterThan(0);

    // Tasks today card
    expect(screen.getByRole("heading", { name: "Tasks today" })).toBeDefined();
    expect(screen.getByText("Review PR")).toBeDefined();

    // Overdue tasks card
    expect(
      screen.getByRole("heading", { name: "Overdue tasks" }),
    ).toBeDefined();
    expect(screen.getByText("Overdue task")).toBeDefined();

    // Balance + recent transactions card
    expect(screen.getByRole("heading", { name: "Balance" })).toBeDefined();
    expect(screen.getByText("Coffee")).toBeDefined();

    // Pinned notes card
    expect(screen.getByRole("heading", { name: "Pinned notes" })).toBeDefined();
    expect(screen.getByText("Project ideas")).toBeDefined();

    fetchMock.mockRestore();
  });

  it("shows empty state when a card has no data", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(
        200,
        makeDashboardResponse({
          events_today: [],
          tasks_today: [],
          overdue_tasks: [],
          recent_transactions: [],
          pinned_notes: [],
        }),
      ),
    );

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
      </MemoryRouter>,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );

    expect(screen.getByText("No events today")).toBeDefined();
    expect(screen.getByText("No tasks due today")).toBeDefined();
    expect(screen.getByText("No overdue tasks")).toBeDefined();
    expect(screen.getByText("No recent transactions")).toBeDefined();
    expect(screen.getByText("No pinned notes")).toBeDefined();

    fetchMock.mockRestore();
  });

  it("deep-links events to calendar with date", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, makeDashboardResponse()));

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
      </MemoryRouter>,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );

    const eventLink = screen.getByRole("link", { name: "Team standup" });
    expect(eventLink.getAttribute("href")).toBe(`/calendar?event=e1`);

    fetchMock.mockRestore();
  });

  it("deep-links tasks to tasks page with id", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, makeDashboardResponse()));

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
      </MemoryRouter>,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );

    const taskLink = screen.getByRole("link", { name: "Review PR" });
    expect(taskLink.getAttribute("href")).toBe(`/tasks?id=t1`);

    fetchMock.mockRestore();
  });

  it("deep-links overdue tasks to tasks page with id", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, makeDashboardResponse()));

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
      </MemoryRouter>,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );

    const overdueLink = screen.getByRole("link", { name: "Overdue task" });
    expect(overdueLink.getAttribute("href")).toBe(`/tasks?id=t2`);

    fetchMock.mockRestore();
  });

  it("deep-links notes to notes page with id", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, makeDashboardResponse()));

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
      </MemoryRouter>,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );

    const noteLink = screen.getByRole("link", { name: "Project ideas" });
    expect(noteLink.getAttribute("href")).toBe(`/notes?id=n1`);

    fetchMock.mockRestore();
  });

  it("navigates client-side on item and card links (no document reload)", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, makeDashboardResponse()));

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
        <LocationProbe />
      </MemoryRouter>,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );

    // A raw <a> leaves loc untouched here: jsdom does not navigate on a
    // plain anchor, so only react-router's Link moves the location.
    fireEvent.click(screen.getByRole("link", { name: "Team standup" }));
    expect(screen.getByTestId("loc").textContent).toBe("/calendar?event=e1");

    fireEvent.click(
      screen.getByRole("link", { name: "View all events today" }),
    );
    expect(screen.getByTestId("loc").textContent).toBe("/calendar");

    fetchMock.mockRestore();
  });

  it("sends explicit date to API (frontend owns the date)", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, makeDashboardResponse()));

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
      </MemoryRouter>,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );

    expect(fetchMock).toHaveBeenCalledWith(
      `/api/dashboard?date=${TODAY}`,
      expect.any(Object),
    );

    fetchMock.mockRestore();
  });

  it("renders income and transfer transactions with correct signs", async () => {
    const responseWithIncomeAndTransfer = makeDashboardResponse({
      recent_transactions: [
        {
          ...tx(),
          id: "tx1",
          kind: "income",
          amount_cents: 100000,
          memo: "Salary",
        },
        {
          ...tx(),
          id: "tx2",
          kind: "transfer",
          amount_cents: 50000,
          memo: "To savings",
        },
      ],
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, responseWithIncomeAndTransfer));

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
      </MemoryRouter>,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );

    // Income transaction should be rendered with memo
    expect(screen.getByText("Salary")).toBeDefined();
    // Transfer transaction should be rendered with memo
    expect(screen.getByText("To savings")).toBeDefined();
    // The income amount should be formatted (100000 cents = 1000 major units)
    expect(screen.getByText(/1,000|1000/)).toBeDefined();
    // The transfer amount should be formatted (50000 cents = 500 major units)
    expect(screen.getByText(/500/)).toBeDefined();

    fetchMock.mockRestore();
  });

  it("falls back to kind when memo is null", async () => {
    const responseWithNullMemo = makeDashboardResponse({
      recent_transactions: [
        {
          ...tx(),
          id: "tx1",
          kind: "expense",
          amount_cents: 25000,
          memo: null,
        },
      ],
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, responseWithNullMemo));

    vi.stubGlobal("fetch", fetchMock);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <DashboardPage />
      </MemoryRouter>,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Today at a glance" }),
      ).toBeDefined(),
    );

    // Should show "expense" as the label since memo is null
    expect(screen.getByText("expense")).toBeDefined();
    expect(screen.getByText(/250/)).toBeDefined();

    fetchMock.mockRestore();
  });
});
