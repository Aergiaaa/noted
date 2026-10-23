import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import TransactionRow from "./TransactionRow";
import type { Transaction } from "../../lib/api";

afterEach(cleanup);

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

const names = new Map([
  ["wal_cash", "Cash"],
  ["wal_bank", "Bank"],
]);

describe("TransactionRow", () => {
  it("routes an expense from its wallet with a minus-signed amount", () => {
    render(
      <table>
        <tbody>
          <TransactionRow tx={tx()} walletNames={names} onVoid={vi.fn()} />
        </tbody>
      </table>,
    );

    expect(screen.getAllByText("Cash →")).toHaveLength(2);
    expect(screen.getByText("−Rp 500")).toBeDefined();
  });

  it("routes income into its wallet with a plus-signed amount", () => {
    render(
      <table>
        <tbody>
          <TransactionRow
            tx={tx({ kind: "income", amount_cents: 200000 })}
            walletNames={names}
            onVoid={vi.fn()}
          />
        </tbody>
      </table>,
    );

    expect(screen.getAllByText("→ Cash")).toHaveLength(2);
    expect(screen.getByText("+Rp 2K")).toBeDefined();
  });

  it("badges a transfer and routes it between both sides", () => {
    render(
      <table>
        <tbody>
          <TransactionRow
            tx={tx({
              kind: "transfer",
              wallet_id: null,
              from_wallet_id: "wal_bank",
              to_wallet_id: "wal_cash",
            })}
            walletNames={names}
            onVoid={vi.fn()}
          />
        </tbody>
      </table>,
    );

    expect(screen.getByText("transfer")).toBeDefined();
    expect(screen.getAllByText("Bank → Cash")).toHaveLength(2);
    expect(screen.getByText("Rp 500")).toBeDefined();
  });

  it("shows the memo beside the kind", () => {
    render(
      <table>
        <tbody>
          <TransactionRow
            tx={tx({ memo: "keyboard" })}
            walletNames={names}
            onVoid={vi.fn()}
          />
        </tbody>
      </table>,
    );

    expect(screen.getByText("expense")).toBeDefined();
    expect(screen.getByText("keyboard")).toBeDefined();
  });

  it("falls back to the raw id for an unknown wallet", () => {
    render(
      <table>
        <tbody>
          <TransactionRow tx={tx()} walletNames={new Map()} onVoid={vi.fn()} />
        </tbody>
      </table>,
    );

    expect(screen.getAllByText("wal_cash →")).toHaveLength(2);
  });

  it("voids only after the inline confirm", () => {
    const onVoid = vi.fn();
    render(
      <table>
        <tbody>
          <TransactionRow tx={tx()} walletNames={names} onVoid={onVoid} />
        </tbody>
      </table>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Void" }));
    expect(onVoid).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Keep" }));
    expect(screen.getByRole("button", { name: "Void" })).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "Void" }));
    fireEvent.click(screen.getByRole("button", { name: "Yes, void" }));
    expect(onVoid).toHaveBeenCalledWith("tx1");
  });
});
