import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import TransferForm from "./TransferForm";
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

function fillValid(amount = "100", date = "2026-10-21", group = ""): void {
  fireEvent.change(screen.getByLabelText("Amount"), {
    target: { value: amount },
  });
  fireEvent.change(screen.getByLabelText("Amount group"), {
    target: { value: group },
  });
  fireEvent.change(screen.getByLabelText("Date"), { target: { value: date } });
}

describe("TransferForm", () => {
  it("defaults from Cash to the first other wallet", () => {
    render(
      <TransferForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    expect((screen.getByLabelText("From") as HTMLInputElement).value).toBe(
      "wal_cash",
    );
    expect((screen.getByLabelText("To") as HTMLInputElement).value).toBe(
      "wal_bank",
    );
    expect((screen.getByLabelText("Date") as HTMLInputElement).value).toBe(
      toDayKey(new Date()),
    );
  });

  it("posts a transfer with the amount converted to cents", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(201, { id: "tx1" }));
    vi.stubGlobal("fetch", fetchMock);
    const onSaved = vi.fn();

    render(
      <TransferForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={onSaved}
      />,
    );

    fillValid("250", "2026-10-21", "75");
    fireEvent.change(screen.getByLabelText("Memo"), {
      target: { value: " top-up " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Move" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/finance/transfers");
    expect(JSON.parse(fetchMock.mock.calls[0][1].body as string)).toEqual({
      from_wallet_id: "wal_cash",
      to_wallet_id: "wal_bank",
      amount_cents: 25007500,
      occurred_on: "2026-10-21",
      memo: "top-up",
    });
  });

  it("posts without a memo key when the memo is empty", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(201, { id: "tx1" }));
    vi.stubGlobal("fetch", fetchMock);

    render(
      <TransferForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    fillValid("50");
    fireEvent.click(screen.getByRole("button", { name: "Move" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body.memo).toBeUndefined();
    expect(body.amount_cents).toBe(5000000);
  });

  it("rejects the same wallet on both sides without calling the server", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    render(
      <TransferForm
        wallets={[wallets[0]]}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    // Explicitly pick both sides (the single-wallet default already
    // matches) so the pickers' handlers run before the client rule trips.
    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "wal_cash" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "wal_cash" },
    });
    fillValid("10");
    fireEvent.click(screen.getByRole("button", { name: "Move" }));

    expect(screen.getByText("Choose two different wallets.")).toBeDefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("rejects an invalid amount without calling the server", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    render(
      <TransferForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    fillValid("0");
    fireEvent.click(screen.getByRole("button", { name: "Move" }));

    expect(screen.getByText("Enter a valid amount.")).toBeDefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("rejects an empty date without calling the server", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    render(
      <TransferForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    fillValid("10", "");
    fireEvent.click(screen.getByRole("button", { name: "Move" }));

    expect(screen.getByText("Date is required.")).toBeDefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("shows server validation errors inline", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(400, {
        error: {
          code: "validation_error",
          message: "invalid",
          fields: { to_wallet_id: "must differ from from_wallet_id" },
        },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    render(
      <TransferForm
        wallets={wallets}
        currency="IDR"
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    fillValid("10");
    fireEvent.click(screen.getByRole("button", { name: "Move" }));

    await waitFor(() =>
      expect(
        screen.getByText("to_wallet_id: must differ from from_wallet_id"),
      ).toBeDefined(),
    );
  });
});
