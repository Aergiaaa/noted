import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import TransactionList from "./TransactionList";
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

const names = new Map([["wal_cash", "Cash"]]);

describe("TransactionList", () => {
  it("renders one row per transaction", () => {
    render(
      <TransactionList
        rows={[tx(), tx({ id: "tx2", memo: "milk" })]}
        walletNames={names}
        onVoid={vi.fn()}
        cursor={null}
        onLoadMore={vi.fn()}
      />,
    );

    expect(screen.getAllByTestId("transaction-row")).toHaveLength(2);
    expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  });

  it("offers Load more only while a cursor remains", () => {
    const onLoadMore = vi.fn();
    render(
      <TransactionList
        rows={[tx()]}
        walletNames={names}
        onVoid={vi.fn()}
        cursor="c1"
        onLoadMore={onLoadMore}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(onLoadMore).toHaveBeenCalledWith("c1");
  });
});
