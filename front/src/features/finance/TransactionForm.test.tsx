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

const wallets: WalletBalance[] = [
  { wallet_id: "wal_cash", name: "Cash", balance_cents: 250000 },
  { wallet_id: "wal_bank", name: "Bank", balance_cents: 1000000 },
];

describe("TransactionForm", () => {
  it("defaults the wallet to Cash and the date to today", () => {
    render(
      <TransactionForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    expect((screen.getByLabelText("Wallet") as HTMLInputElement).value).toBe(
      "wal_cash",
    );
    expect((screen.getByLabelText("Date") as HTMLInputElement).value).toBe(
      toDayKey(new Date()),
    );
  });

  it("posts income with the amount converted to cents", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(201, { id: "tx1" }));
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
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/finance/transactions");
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body).toEqual({
      kind: "income",
      wallet_id: "wal_bank",
      amount_cents: 5005500,
      occurred_on: "2026-10-21",
      memo: "keyboard",
    });
  });

  it("posts an expense without a memo key when the memo is empty", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(201, { id: "tx1" }));
    vi.stubGlobal("fetch", fetchMock);

    render(
      <TransactionForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "12" },
    });
    fireEvent.change(screen.getByLabelText("Date"), {
      target: { value: "2026-10-21" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body).toEqual({
      kind: "expense",
      wallet_id: "wal_cash",
      amount_cents: 1200000,
      occurred_on: "2026-10-21",
    });
    expect("memo" in body).toBe(false);
  });

  it("posts a group-only amount as a sub-thousand value", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(201, { id: "tx1" }));
    vi.stubGlobal("fetch", fetchMock);

    render(
      <TransactionForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText("Amount group"), {
      target: { value: "500" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body.amount_cents).toBe(50000);
  });

  it("rejects a group longer than three digits without calling the server", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    render(
      <TransactionForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "200" },
    });
    fireEvent.change(screen.getByLabelText("Amount group"), {
      target: { value: "1234" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    expect(screen.getByText("Enter a valid amount.")).toBeDefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("rejects an invalid amount without calling the server", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    render(
      <TransactionForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "abc" },
    });
    fireEvent.change(screen.getByLabelText("Date"), {
      target: { value: "2026-10-21" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    expect(screen.getByRole("alert")).toBeDefined();
    expect(screen.getByText("Enter a valid amount.")).toBeDefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("rejects an empty date without calling the server", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    render(
      <TransactionForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "10" },
    });
    fireEvent.change(screen.getByLabelText("Date"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Record" }));

    expect(screen.getByText("Date is required.")).toBeDefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("shows server validation errors inline", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(400, {
        error: {
          code: "validation_error",
          message: "invalid",
          fields: { wallet_id: "unknown wallet" },
        },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    render(
      <TransactionForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

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
