import { useState } from "react";
import type { Transaction } from "../../lib/api";
import { formatTransactionRouting } from "../../lib/format";
import MoneyBadge from "./MoneyBadge";

interface TransactionRowProps {
  tx: Transaction;
  walletNames: Map<string, string>;
  onVoid: (id: string) => void;
}

// One history line: date, kind/memo, wallet routing, compact signed
// amount (full value on hover), void behind an inline confirm — money
// voids are destructive (UI.md: every destructive action confirms).
export default function TransactionRow({
  tx,
  walletNames,
  onVoid,
}: TransactionRowProps) {
  const [confirming, setConfirming] = useState(false);

  const routing = formatTransactionRouting(tx, walletNames);
  const sign =
    tx.kind === "income" ? "+" : tx.kind === "expense" ? "−" : undefined;

  return (
    <tr data-testid="transaction-row">
      <td className="w-px whitespace-nowrap tabular-nums">{tx.occurred_on}</td>
      <td>
        <div className="flex items-center gap-2">
          {tx.kind === "transfer" ? (
            <span className="badge badge-info badge-sm">transfer</span>
          ) : (
            <span className="text-xs text-base-content/60">{tx.kind}</span>
          )}
          {tx.memo !== null && <span className="text-sm">{tx.memo}</span>}
        </div>
        <div className="mt-1 text-xs text-base-content/60 md:hidden">
          {routing}
        </div>
      </td>
      <td className="hidden whitespace-nowrap text-base-content/70 md:table-cell">
        {routing}
      </td>
      <td className="whitespace-nowrap text-right">
        <MoneyBadge
          cents={tx.amount_cents}
          currency={tx.currency}
          sign={sign}
          compact
        />
      </td>
      <td className="w-px">
        {confirming ? (
          <div className="flex items-center justify-end gap-1">
            <button
              type="button"
              className="btn btn-ghost btn-xs"
              onClick={() => setConfirming(false)}
            >
              Keep
            </button>
            <button
              type="button"
              className="btn btn-error btn-xs"
              onClick={() => onVoid(tx.id)}
            >
              Yes, void
            </button>
          </div>
        ) : (
          <button
            type="button"
            className="btn btn-ghost btn-xs"
            onClick={() => setConfirming(true)}
          >
            Void
          </button>
        )}
      </td>
    </tr>
  );
}
