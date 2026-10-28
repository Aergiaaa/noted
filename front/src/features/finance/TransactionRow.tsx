import { useState } from "react";
import type { Transaction } from "../../lib/api";
import type { MentionType } from "../../lib/mentions";
import { formatTransactionRouting } from "../../lib/format";
import LinkChip, { type LinkField } from "../links/LinkChip";
import MoneyBadge from "./MoneyBadge";

interface TransactionRowProps {
  tx: Transaction;
  walletNames: Map<string, string>;
  onVoid: (id: string) => void;
  linkTitle?: (kind: MentionType, id: string) => string | undefined;
  // Transactions have no edit form, so unlinking happens here — behind
  // the chip's own confirm (F9).
  onUnlink?: (id: string, field: LinkField) => void;
}

// One history line: date, kind/memo, wallet routing, compact signed
// amount (full value on hover), void behind an inline confirm — money
// voids are destructive (UI.md: every destructive action confirms).
export default function TransactionRow({
  tx,
  walletNames,
  onVoid,
  linkTitle,
  onUnlink,
}: TransactionRowProps) {
  const [confirming, setConfirming] = useState(false);

  const routing = formatTransactionRouting(tx, walletNames);
  const sign =
    tx.kind === "income" ? "+" : tx.kind === "expense" ? "−" : undefined;
  const hasLinks =
    tx.task_id !== null || tx.event_id !== null || tx.note_id !== null;

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
        {hasLinks && (
          <div className="mt-1 flex flex-wrap gap-1">
            {tx.task_id !== null && (
              <LinkChip
                kind="task"
                id={tx.task_id}
                title={linkTitle?.("task", tx.task_id)}
                onUnlink={onUnlink && ((field) => onUnlink(tx.id, field))}
              />
            )}
            {tx.event_id !== null && (
              <LinkChip
                kind="event"
                id={tx.event_id}
                title={linkTitle?.("event", tx.event_id)}
                onUnlink={onUnlink && ((field) => onUnlink(tx.id, field))}
              />
            )}
            {tx.note_id !== null && (
              <LinkChip
                kind="note"
                id={tx.note_id}
                title={linkTitle?.("note", tx.note_id)}
                onUnlink={onUnlink && ((field) => onUnlink(tx.id, field))}
              />
            )}
          </div>
        )}
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
