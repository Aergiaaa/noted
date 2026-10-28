import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import TransactionForm from "./TransactionForm";
import type { WalletBalance } from "../../lib/api";
import { toDayKey } from "../../lib/format";

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

function fetchRouter(saveStatus = 201) {
  return vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    if (init?.method === "POST") {
      return Promise.resolve(jsonResponse(saveStatus, { id: "tx1" }));
    }
    if (url.includes("/tasks")) {
      return Promise.resolve(
        jsonResponse(200, {
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
        }),
      );
    }
    if (url.includes("/events?")) {
      return Promise.resolve(
        jsonResponse(200, {
          data: [
            {
              id: "e9",
              title: "Dentist",
              starts_at: "2026-10-15T09:00:00Z",
              ends_at: "2026-10-15T10:00:00Z",
            },
          ],
        }),
      );
    }
    if (url.includes("/notes")) {
      return Promise.resolve(
        jsonResponse(200, {
          data: [{ id: "n9", title: "Groceries", body_md: "", pinned: false }],
        }),
      );
    }
    return Promise.resolve(jsonResponse(200, { data: [] }));
  });
}

const wallets: WalletBalance[] = [
  { wallet_id: "wal_cash", name: "Cash", balance_cents: 250000 },
  { wallet_id: "wal_bank", name: "Bank", balance_cents: 1000000 },
];

function renderForm() {
  return render(
    <TransactionForm
      wallets={wallets}
      currency="IDR"
      onClose={vi.fn()}
      onSaved={vi.fn()}
    />,
  );
}

async function pickersLoaded() {
  await waitFor(() =>
    expect(screen.getByRole("option", { name: "Buy keyboard" })).toBeDefined(),
  );
  expect(screen.getByRole("option", { name: "Dentist" })).toBeDefined();
  expect(screen.getByRole("option", { name: "Groceries" })).toBeDefined();
}

describe("TransactionForm", () => {
  it("defaults the wallet to Cash and the date to today", () => {
    renderForm();

    expect((screen.getByLabelText("Wallet") as HTMLInputElement).value).toBe(
      "wal_cash",
    );
    expect((screen.getByLabelText("Date") as HTMLInputElement).value).toBe(
      toDayKey(new Date()),
    );
  });

  it("offers only existing items in the task, event and note pickers", async () => {
    vi.stubGlobal("fetch", fetchRouter());
    renderForm();
    await pickersLoaded();
    expect(screen.getAllByRole("option", { name: "None" })).toHaveLength(3);
  });

  it("posts income with the amount converted to cents and no link keys when unset", async () => {
    const fetchMock = fetchRouter();
    vi.stubGlobal("fetch", fetchMock);
    const onSaved = vi.fn();

    render(
      <TransactionForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={onSaved}
      />,
    );
    await pickersLoaded();

    fireEvent.change(screen.getByLabelText("Kind"), {
      target: { value: "income" },
    });
    fireEvent.change(screen.getByLabelText("Wallet"), {
      target: { value: "wal_bank" },
    });
    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "50" },
    });
    fireEvent.change(screen.getByLabelText("Amount group"), {
      target: { value: "55" },
    });
    fireEvent.change(screen.getByLabelText("Date"), {
      target: { value: "2026-10-21" },
    });
    fireEvent.change(screen.getByLabelText("Memo"), {
      target: { value: "  keyboard  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    const post = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    expect(post?.[0]).toBe("/api/finance/transactions");
    const body = JSON.parse((post![1] as { body: string }).body);
    expect(body).toEqual({
      kind: "income",
      wallet_id: "wal_bank",
      amount_cents: 5005500,
      occurred_on: "2026-10-21",
      memo: "keyboard",
    });
  });

  it("sends picked link ids on create", async () => {
    const fetchMock = fetchRouter();
    vi.stubGlobal("fetch", fetchMock);
    renderForm();
    await pickersLoaded();

    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "12" },
    });
    fireEvent.change(screen.getByLabelText("Task"), {
      target: { value: "t1" },
    });
    fireEvent.change(screen.getByLabelText("Event"), {
      target: { value: "e9" },
    });
    fireEvent.change(screen.getByLabelText("Note"), {
      target: { value: "n9" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    await waitFor(() => {
      const post = fetchMock.mock.calls.find(
        (call) => (call[1] as RequestInit)?.method === "POST",
      );
      expect(post).toBeDefined();
    });
    const post = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    const body = JSON.parse((post![1] as { body: string }).body);
    expect(body.task_id).toBe("t1");
    expect(body.event_id).toBe("e9");
    expect(body.note_id).toBe("n9");
  });

  it("posts an expense without a memo key when the memo is empty", async () => {
    const fetchMock = fetchRouter();
    vi.stubGlobal("fetch", fetchMock);

    renderForm();
    await pickersLoaded();

    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "12" },
    });
    fireEvent.change(screen.getByLabelText("Date"), {
      target: { value: "2026-10-21" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    await waitFor(() => {
      const post = fetchMock.mock.calls.find(
        (call) => (call[1] as RequestInit)?.method === "POST",
      );
      expect(post).toBeDefined();
    });
    const post = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    const body = JSON.parse((post![1] as { body: string }).body);
    expect(body).toEqual({
      kind: "expense",
      wallet_id: "wal_cash",
      amount_cents: 1200000,
      occurred_on: "2026-10-21",
    });
    expect("memo" in body).toBe(false);
  });

  it("posts a group-only amount as a sub-thousand value", async () => {
    const fetchMock = fetchRouter();
    vi.stubGlobal("fetch", fetchMock);

    renderForm();
    await pickersLoaded();

    fireEvent.change(screen.getByLabelText("Amount group"), {
      target: { value: "500" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    await waitFor(() => {
      const post = fetchMock.mock.calls.find(
        (call) => (call[1] as RequestInit)?.method === "POST",
      );
      expect(post).toBeDefined();
    });
    const post = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    const body = JSON.parse((post![1] as { body: string }).body);
    expect(body.amount_cents).toBe(50000);
  });

  it("rejects a group longer than three digits without calling the server", async () => {
    const fetchMock = fetchRouter();
    vi.stubGlobal("fetch", fetchMock);

    renderForm();
    await pickersLoaded();

    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "200" },
    });
    fireEvent.change(screen.getByLabelText("Amount group"), {
      target: { value: "1234" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    expect(screen.getByText("Enter a valid amount.")).toBeDefined();
    const posts = fetchMock.mock.calls.filter(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    expect(posts).toHaveLength(0);
  });

  it("rejects an invalid amount without calling the server", async () => {
    const fetchMock = fetchRouter();
    vi.stubGlobal("fetch", fetchMock);

    renderForm();
    await pickersLoaded();

    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "abc" },
    });
    fireEvent.change(screen.getByLabelText("Date"), {
      target: { value: "2026-10-21" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    expect(screen.getByRole("alert")).toBeDefined();
    expect(screen.getByText("Enter a valid amount.")).toBeDefined();
    const posts = fetchMock.mock.calls.filter(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    expect(posts).toHaveLength(0);
  });

  it("rejects an empty date without calling the server", async () => {
    const fetchMock = fetchRouter();
    vi.stubGlobal("fetch", fetchMock);

    renderForm();
    await pickersLoaded();

    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "10" },
    });
    fireEvent.change(screen.getByLabelText("Date"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    expect(screen.getByText("Date is required.")).toBeDefined();
    const posts = fetchMock.mock.calls.filter(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    expect(posts).toHaveLength(0);
  });

  it("shows server validation errors inline", async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation((_url: string, init?: RequestInit) => {
        if (init?.method === "POST") {
          return Promise.resolve(
            jsonResponse(400, {
              error: {
                code: "validation_error",
                message: "invalid",
                fields: { wallet_id: "unknown wallet" },
              },
            }),
          );
        }
        return Promise.resolve(jsonResponse(200, { data: [] }));
      });
    vi.stubGlobal("fetch", fetchMock);

    renderForm();

    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "10" },
    });
    fireEvent.change(screen.getByLabelText("Date"), {
      target: { value: "2026-10-21" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    await waitFor(() =>
      expect(screen.getByText("wallet_id: unknown wallet")).toBeDefined(),
    );
  });
});
