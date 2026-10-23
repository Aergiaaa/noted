import type { Transaction } from "../../lib/api";
import TransactionRow from "./TransactionRow";

interface TransactionListProps {
  rows: Transaction[];
  walletNames: Map<string, string>;
  onVoid: (id: string) => void;
  cursor: string | null;
  onLoadMore: (cursor: string) => void;
}

// Dense history table; the page owns the empty/loading/error states and
// hands the list rows to render (F5 list pattern).
export default function TransactionList({
  rows,
  walletNames,
  onVoid,
  cursor,
  onLoadMore,
}: TransactionListProps) {
  return (
    <div className="overflow-x-auto">
      <table className="table table-sm">
        <thead>
          <tr>
            <th>Date</th>
            <th>Details</th>
            <th className="hidden md:table-cell">Wallets</th>
            <th className="text-right">Amount</th>
            <th aria-label="Actions" />
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <TransactionRow
              key={row.id}
              tx={row}
              walletNames={walletNames}
              onVoid={onVoid}
            />
          ))}
        </tbody>
      </table>
      {cursor !== null && (
        <div className="mt-3 flex justify-center">
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={() => onLoadMore(cursor)}
          >
            Load more
          </button>
        </div>
      )}
    </div>
  );
}
